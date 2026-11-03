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
  ($Exception.HResult -band 0xffff) -in @(11, 32, 33)
}
function Get-DinahQueueFiles([string]$Directory) {
  @(Get-ChildItem -LiteralPath $Directory -File | Where-Object { $_.Name -match '^\d{19}-[a-f0-9]{32}\.ticket$' })
}
function Open-DinahQueueAllocator([string]$Directory, $Cancellation = $null) {
  $allocator = $null
  while (-not $allocator) {
    if ($Cancellation -and $Cancellation.IsCompleted) { throw [OperationCanceledException]::new('The waiting suite exited.') }
    try { $allocator = [IO.File]::Open((Join-Path $Directory 'sequence'), [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None) }
    catch [IO.IOException] { if (-not (Test-DinahSharingViolation $_.Exception)) { throw }; Start-Sleep -Milliseconds 25 }
  }
  return $allocator
}
function New-DinahQueueTicket([string]$Directory, $Cancellation) {
  [IO.Directory]::CreateDirectory($Directory) | Out-Null
  $allocator = Open-DinahQueueAllocator $Directory $Cancellation
  try {
    [long]$last = 0
    foreach ($file in (Get-DinahQueueFiles $Directory)) { $last = [Math]::Max($last, [long]$file.Name.Substring(0, 19)) }
    # A killed writer can leave a partial sequence. Existing tickets remain
    # the lower bound, even when the persisted value cannot be read.
    if ($allocator.Length -eq 8) {
      $bytes = [byte[]]::new(8)
      if ($allocator.Read($bytes, 0, 8) -eq 8) { $last = [Math]::Max($last, [BitConverter]::ToInt64($bytes, 0)) }
    }
    while ($true) {
      if ($last -eq [long]::MaxValue) { throw 'The browser test queue sequence is exhausted.' }
      $last++
      $bytes = [BitConverter]::GetBytes([long]$last)
      $allocator.Position = 0
      $allocator.Write($bytes, 0, 8)
      $allocator.SetLength(8)
      $allocator.Flush($true)
      $name = $last.ToString('D19') + '-' + [Guid]::NewGuid().ToString('N') + '.ticket'
      $ticketPath = Join-Path $Directory $name
      try {
        $handle = [IO.FileStream]::new($ticketPath, [IO.FileMode]::CreateNew, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None, 1, [IO.FileOptions]::DeleteOnClose)
        return @{ Path = $ticketPath; Number = $last; Handle = $handle }
      } catch [UnauthorizedAccessException] { continue }
      catch [IO.IOException] {
        if (($_.Exception.HResult -band 0xffff) -notin @(17, 80, 183, 5)) { throw }
      }
    }
  } finally { $allocator.Dispose() }
}
function Get-DinahQueuePosition([string]$Directory, $Ticket, $Cancellation = $null) {
  # On Unix an open and its advisory lock are separate OS operations. Hold
  # the allocator while probing, so a just-created ticket cannot be mistaken
  # for an abandoned file before its creator has acquired that lock.
  $allocator = Open-DinahQueueAllocator $Directory $Cancellation
  try {
    $position = 1
    foreach ($file in (Get-DinahQueueFiles $Directory)) {
      if ($file.FullName -eq $Ticket.Path) { continue }
      $probe = $null
      try {
        # All participants hold FileShare.None. A successful exclusive open
        # proves this is an abandoned ticket, including after SIGKILL on Unix.
        $probe = [IO.FileStream]::new($file.FullName, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None, 1, [IO.FileOptions]::DeleteOnClose)
        if (-not $IsWindows) { [IO.File]::Delete($file.FullName) }
      } catch [IO.FileNotFoundException] { }
      catch [UnauthorizedAccessException] {
        if ([long]$file.Name.Substring(0, 19) -lt $Ticket.Number) { $position++ }
      } catch [IO.IOException] {
        if (-not (Test-DinahSharingViolation $_.Exception)) { throw }
        if ([long]$file.Name.Substring(0, 19) -lt $Ticket.Number) { $position++ }
      } finally { if ($probe) { $probe.Dispose() } }
    }
    $position
  } finally { $allocator.Dispose() }
}
function Enter-DinahBrowserLock([string]$Path, [string]$Description, $Cancellation = $null) {
  $Path = [IO.Path]::GetFullPath($Path)
  [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($Path)) | Out-Null
  $directory = [IO.Path]::ChangeExtension($Path, '.queue')
  $queueTicket = New-DinahQueueTicket $directory $Cancellation
  $nextNotice = [DateTime]::MinValue
  $stream = $null
  try {
    while ($true) {
      if ($Cancellation -and $Cancellation.IsCompleted) { throw [OperationCanceledException]::new('The waiting suite exited.') }
      $position = Get-DinahQueuePosition $directory $queueTicket $Cancellation
      if ($position -eq 1) {
        try {
          $stream = [IO.File]::Open($Path, [IO.FileMode]::OpenOrCreate, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
          try {
            $lease = @{ path = $Path; token = [Guid]::NewGuid().ToString('N'); pid = $PID; queue = $queueTicket.Path; owner = $Description; since = [DateTime]::UtcNow.ToString('o') }
            [IO.File]::WriteAllText($Path + '.owner.json', ($lease | ConvertTo-Json -Compress))
            # Keep the existing Stream.Dispose() caller contract, disposing the
            # lock before its queue ticket so the next waiter can advance.
            $resource = [pscustomobject]@{ Lock = $stream; Queue = $queueTicket.Handle }
            $resource | Add-Member ScriptMethod Dispose { try { $this.Lock.Dispose() } finally { $this.Queue.Dispose() } }
            return @{ Stream = $resource; Ticket = ($lease | ConvertTo-Json -Compress) }
          } catch { $stream.Dispose(); throw }
        } catch [IO.IOException] {
          if (-not (Test-DinahSharingViolation $_.Exception)) { throw }
        }
      }
      if ([DateTime]::UtcNow -ge $nextNotice) {
        $holder = 'another browser test run (owner information is being written)'
        try {
          $v = [IO.File]::ReadAllText($Path + '.owner.json') | ConvertFrom-Json
          $since = ([DateTime]$v.since).ToUniversalTime().ToString('o')
          $holder = "$($v.owner), PID $($v.pid), since $since"
        } catch { }
        [Console]::Error.WriteLine("Waiting for browser test lock: queue position $position; ticket $($queueTicket.Number); holder $holder")
        $nextNotice = [DateTime]::UtcNow.AddSeconds(5)
      }
      Start-Sleep -Milliseconds 250
    }
  } catch { if ($stream) { $stream.Dispose() }; $queueTicket.Handle.Dispose(); throw }
}
function Test-DinahBrowserTicket([string]$Path, [string]$Value) {
  $Path = [IO.Path]::GetFullPath($Path)
  if (-not $Value) { return $false }
  try {
    $given = $Value | ConvertFrom-Json
    $held = [IO.File]::ReadAllText($Path + '.owner.json') | ConvertFrom-Json
    if ($given.path -ne $Path -or $given.token -ne $held.token -or -not $held.queue -or $given.queue -ne $held.queue) { return $false }
    try {
      $probe = [IO.File]::Open($held.queue, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
      $probe.Dispose()
      return $false
    } catch [IO.IOException] { if (-not (Test-DinahSharingViolation $_.Exception)) { return $false } }
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
  # Watch stdin while queued too. A dead Node waiter must release its ticket
  # without first reaching the head of the queue.
  $released = [Console]::OpenStandardInput().ReadAsync((New-Object byte[] 1), 0, 1)
  $lease = Enter-DinahBrowserLock $LockPath $Owner $released
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
    if (-not $released.Wait([TimeSpan]::FromSeconds($MaxSeconds))) {
      [Console]::Error.WriteLine("Browser test lock held for over $MaxSeconds seconds by $Owner; releasing it and stopping that suite.")
      $lease.Stream.Dispose()
      exit 3
    }
  } finally { $lease.Stream.Dispose() }
}
