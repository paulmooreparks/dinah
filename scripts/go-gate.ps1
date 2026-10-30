# Runs go test for a package-wide or full gate under the machine-wide lock the browser suites use, so no two gates
# overlap whoever starts them. It waits for the lock, printing who holds it every five seconds, runs `go test` with
# the arguments it is given, releases the lock, and exits with go test's exit code. A single-test run may skip it.
#
#   pwsh scripts/go-gate.ps1 ./cmd/dinah-desktop ./internal/... -count=1
#
# Run from the repository root. A run inside a browser batch (which passes its ticket in DINAH_BROWSER_TEST_TICKET)
# borrows the batch's lock instead of waiting on it.
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$GoTestArgs)
$ErrorActionPreference = 'Stop'
if (-not $GoTestArgs -or $GoTestArgs.Count -eq 0) { [Console]::Error.WriteLine('Usage: go-gate.ps1 <go test arguments>'); exit 2 }
. (Join-Path $PSScriptRoot '../cmd/dinah-desktop/frontend/e2e/lib/browser-lock.ps1')
$lockPath = Get-DinahBrowserLockPath
$borrowed = Test-DinahBrowserTicket $lockPath $env:DINAH_BROWSER_TEST_TICKET
$lease = $null
if (-not $borrowed) {
  $lease = Enter-DinahBrowserLock $lockPath ('go test ' + ($GoTestArgs -join ' ') + ' in ' + (Get-Location).Path)
}
try {
  & go test @GoTestArgs
  exit $LASTEXITCODE
} finally {
  if ($lease) { $lease.Stream.Dispose() }
}
