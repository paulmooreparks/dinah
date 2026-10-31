# FileShare.None makes this an OS-held lock, independent of file existence.
param([string]$LockPath, [string]$Owner = 'browser suite', [string]$Ticket)
$ErrorActionPreference = 'Stop'

# The lock is %ProgramData%/Dinah/browser-tests.lock on Windows. Elsewhere the common application data folder is
# /usr/share, which an ordinary user cannot write, so it is /tmp/Dinah/browser-tests.lock. DINAH_BROWSER_LOCK names
# another file on any system. lib/browser-lock.js chooses the same file.
function Get-DinahBrowserLockPath {
  if ($env:DINAH_BROWSER_LOCK) { return [IO.Path]::GetFullPath($env:DINAH_BROWSER_LOCK) }
  $base = if ($IsWindows) { [Environment]::GetFolderPath('CommonApplicationData') } else { '/tmp' }
  [IO.Path]::GetFullPath((Join-Path $base 'Dinah/browser-tests.lock'))
}
# On Windows a held FileShare.None handle refuses another open with ERROR_SHARING_VIOLATION (32) or
# ERROR_LOCK_VIOLATION (33). On Linux and macOS .NET holds FileShare.None as an exclusive flock, and a refused open
# carries flock's errno, EWOULDBLOCK: 11 on Linux, 35 on macOS.
function Test-DinahSharingViolation($Exception) {
  while ($Exception.InnerException) { $Exception = $Exception.InnerException }
  $codes = if ($IsWindows) { @(32, 33) } elseif ($IsMacOS) { @(35) } else { @(11) }
  ($Exception.HResult -band 0xffff) -in $codes
}
# A process's start, which tells it from a later process given the same ID. Windows and macOS report it exactly, but
# on Linux .NET derives StartTime from the boot time, which it recomputes in each process, so two processes read
# times a few milliseconds apart for the same process. There the start is read as /proc/<pid>/stat reports it, in
# clock ticks since boot (field 22, counted after the command name, which may itself hold spaces or parentheses).
function Get-DinahProcessBirth([int]$Id) {
  if ($IsLinux) {
    $stat = [IO.File]::ReadAllText("/proc/$Id/stat")
    return ($stat.Substring($stat.LastIndexOf(')') + 2) -split ' ')[19]
  }
  (Get-Process -Id $Id -ErrorAction Stop).StartTime.ToUniversalTime().Ticks.ToString()
}
function Enter-DinahBrowserLock([string]$Path, [string]$Description) {
  $Path = [IO.Path]::GetFullPath($Path)
  [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path)) | Out-Null
  $nextNotice = [DateTime]::MinValue
  while ($true) {
    try {
      $stream = [IO.File]::Open($Path, [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
      try {
        $lease = @{ path = $Path; token = [Guid]::NewGuid().ToString('N'); pid = $PID; birth = (Get-DinahProcessBirth $PID); owner = $Description; since = [DateTime]::UtcNow.ToString('o') }
        [IO.File]::WriteAllText($Path + '.owner.json', ($lease | ConvertTo-Json -Compress))
        return @{ Stream = $stream; Ticket = ($lease | ConvertTo-Json -Compress) }
      } catch { $stream.Dispose(); throw }
    } catch [IO.IOException] {
      if (-not (Test-DinahSharingViolation $_.Exception)) { throw }
      if ([DateTime]::UtcNow -ge $nextNotice) {
        $holder = 'another browser test run (owner information is being written)'
        try {
          $v = [IO.File]::ReadAllText($Path + '.owner.json') | ConvertFrom-Json
          $since = ([DateTime]$v.since).ToUniversalTime().ToString('o')
          $holder = "$($v.owner), PID $($v.pid), since $since"
        } catch { }
        [Console]::Error.WriteLine("Waiting for browser test lock: $holder")
        $nextNotice = [DateTime]::UtcNow.AddSeconds(5)
      }
      Start-Sleep -Milliseconds 250
    }
  }
}
function Test-DinahBrowserTicket([string]$Path, [string]$Value) {
  $Path = [IO.Path]::GetFullPath($Path)
  if (-not $Value) { return $false }
  try {
    $given = $Value | ConvertFrom-Json
    $held = [IO.File]::ReadAllText($Path + '.owner.json') | ConvertFrom-Json
    if ($given.path -ne $Path -or $given.token -ne $held.token -or $given.pid -ne $held.pid -or $given.birth -ne $held.birth) { return $false }
    if ((Get-DinahProcessBirth $held.pid) -ne $held.birth) { return $false }
    try {
      $probe = [IO.File]::Open($Path, [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
      $probe.Dispose()
      return $false
    } catch [IO.IOException] { return (Test-DinahSharingViolation $_.Exception) }
  } catch { return $false }
}
# Dot-sourcing supplies the same acquisition functions to run.ps1. A direct
# node suite keeps this helper's stdin open for its browser's lifetime.
if ($MyInvocation.InvocationName -ne '.') {
  if (-not $LockPath) { $LockPath = Get-DinahBrowserLockPath }
  if (Test-DinahBrowserTicket $LockPath $Ticket) {
    [Console]::Out.WriteLine('DINAH_BROWSER_LOCK_READY borrowed')
    exit 0
  }
  $lease = Enter-DinahBrowserLock $LockPath $Owner
  try {
    [Console]::Out.WriteLine('DINAH_BROWSER_LOCK_READY acquired')
    [Console]::Out.Flush()
    [Console]::In.ReadLine() | Out-Null
  } finally { $lease.Stream.Dispose() }
}
