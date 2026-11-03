# Run any test/build command through the same FIFO queue as browser and Go
# batches. Never open browser-tests.lock directly to run a command.
# Keep command flags out of PowerShell's advanced parameter binder: a
# command's -e, -Out or -ErrorAction belongs to that command, not this script.
param()
$ErrorActionPreference = 'Stop'
$commandLine = @($args)
$requestedLockPath = $null
if ($commandLine.Count -ge 2 -and $commandLine[0] -eq '-LockPath') {
  $requestedLockPath = $commandLine[1]
  $commandLine = @($commandLine | Select-Object -Skip 2)
}
if (-not $commandLine.Count) { throw 'Usage: with-test-lock.ps1 [-LockPath path] command arguments' }
$Command = $commandLine[0]
$CommandArgs = @($commandLine | Select-Object -Skip 1)
. (Join-Path $PSScriptRoot '../cmd/dinah-desktop/frontend/e2e/lib/browser-lock.ps1')
$LockPath = if ($requestedLockPath) { $requestedLockPath } else { Get-DinahBrowserLockPath }
$previousTicket = $env:DINAH_BROWSER_TEST_TICKET
$lease = $null
try {
  if (-not (Test-DinahBrowserTicket $LockPath $previousTicket)) {
    $lease = Enter-DinahBrowserLock $LockPath ($Command + ' ' + ($CommandArgs -join ' '))
    $env:DINAH_BROWSER_TEST_TICKET = $lease.Ticket
  }
  & $Command @CommandArgs
  exit $LASTEXITCODE
} finally {
  $env:DINAH_BROWSER_TEST_TICKET = $previousTicket
  if ($lease) { $lease.Stream.Dispose() }
}
