param(
  [string]$EventName = $env:GITHUB_EVENT_NAME,
  [string]$SkipTest = $env:DINAH_ONE_OFF_SKIP,
  [switch]$Describe
)
$ErrorActionPreference = 'Stop'
if (-not $SkipTest) { $SkipTest = 'none' }
$testArgs = @('./...', '-count=1')
switch -CaseSensitive ($SkipTest) {
  'none' { }
  'TestCtrlCInterruptsNodeServerInPowerShell' {
    if ($EventName -cne 'workflow_dispatch') { throw 'A test exception requires manual workflow dispatch.' }
    $testArgs += @('-skip', '^TestCtrlCInterruptsNodeServerInPowerShell$')
  }
  default { throw 'Only the approved Node pre-output test may be excepted.' }
}
if ($Describe) {
  ConvertTo-Json -Compress -InputObject $testArgs
  exit 0
}
& (Join-Path $PSScriptRoot 'go-gate.ps1') @testArgs
exit $LASTEXITCODE
