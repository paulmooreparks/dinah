# FileShare.None makes this an OS-held lock, independent of file existence.
# MaxSeconds bounds a direct suite's lease: the operating system frees the
# lock when its holder exits, but a suite that is alive and stuck would hold
# it for every other run. run.ps1 batches and Go gates take the lock through
# the functions below and keep their own limits.
param([string]$LockPath, [string]$Owner = 'browser suite', [string]$Ticket, [int]$MaxSeconds = 900)
$ErrorActionPreference = 'Stop'

function Get-DinahBrowserLockPath {
  # On Linux and macOS .NET maps CommonApplicationData to /usr/share, which
  # an ordinary user cannot write, so the Go gate there locks in the shared
  # temporary folder. The browser suites themselves run on Windows only.
  $root = if ($IsWindows) { [Environment]::GetFolderPath('CommonApplicationData') } else { [IO.Path]::GetTempPath() }
  [IO.Path]::GetFullPath((Join-Path $root 'Dinah/browser-tests.lock'))
}
function Test-DinahSharingViolation($Exception) {
  while ($Exception.InnerException) { $Exception = $Exception.InnerException }
  ($Exception.HResult -band 0xffff) -in @(32, 33)
}
function Enter-DinahBrowserLock([string]$Path, [string]$Description) {
  $Path = [IO.Path]::GetFullPath($Path)
  [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path)) | Out-Null
  $nextNotice = [DateTime]::MinValue
  while ($true) {
    try {
      $stream = [IO.File]::Open($Path, [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
      try {
        $lease = @{ path = $Path; token = [Guid]::NewGuid().ToString('N'); pid = $PID; birth = (Get-Process -Id $PID).StartTime.ToUniversalTime().Ticks.ToString(); owner = $Description; since = [DateTime]::UtcNow.ToString('o') }
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
    $ownerProcess = Get-Process -Id $held.pid -ErrorAction Stop
    if ($ownerProcess.StartTime.ToUniversalTime().Ticks.ToString() -ne $held.birth) { return $false }
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
    # The suite writes a line, or closes stdin by exiting, to release. One
    # that has held the lock past MaxSeconds is taken to be stuck: the lock
    # is released and this helper exits, and the suite stops itself when it
    # sees its holder gone (browser-lock.js).
    # Console.In is a synchronized reader, whose ReadLineAsync completes
    # synchronously and so cannot time out; the raw input stream's read runs
    # apart from this thread. Any byte, or the end of input, releases.
    $released = [Console]::OpenStandardInput().ReadAsync((New-Object byte[] 1), 0, 1)
    if (-not $released.Wait([TimeSpan]::FromSeconds($MaxSeconds))) {
      [Console]::Error.WriteLine("Browser test lock held for over $MaxSeconds seconds by $Owner; releasing it and stopping that suite.")
      $lease.Stream.Dispose()
      exit 3
    }
  } finally { $lease.Stream.Dispose() }
}
