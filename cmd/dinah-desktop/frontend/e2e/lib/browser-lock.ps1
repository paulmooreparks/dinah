# FileShare.None makes this an OS-held lock, independent of file existence.
param([string]$LockPath, [string]$Owner = 'browser suite', [string]$Ticket)
$ErrorActionPreference = 'Stop'

function Get-DinahBrowserLockPath {
  [IO.Path]::GetFullPath((Join-Path ([Environment]::GetFolderPath('CommonApplicationData')) 'Dinah/browser-tests.lock'))
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
    [Console]::In.ReadLine() | Out-Null
  } finally { $lease.Stream.Dispose() }
}
