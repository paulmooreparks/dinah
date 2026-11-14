param([Parameter(Mandatory=$true)][string]$YesterdayTree,[Parameter(Mandatory=$true)][string]$Out,[string]$CurrentTest='^TestBusScanMigrationTrial$')
$ErrorActionPreference='Stop'
$outPath=[IO.Path]::GetFullPath($Out)
$tempRoot=[IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')+'\'
if (-not $outPath.StartsWith($tempRoot,[StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath $outPath)) { throw 'Supply a new child folder of TEMP.' }
$oldTree=[IO.Path]::GetFullPath($YesterdayTree)
$revision=& git -C $oldTree rev-parse HEAD
if ($LASTEXITCODE -ne 0 -or -not $revision.StartsWith('4fe5a52c')) { throw 'YesterdayTree must be the separate 4fe5a52c checkout.' }
$testPath=Join-Path $oldTree 'internal/bus/scan_yesterday_external_test.go'
if (Test-Path -LiteralPath $testPath) { throw 'Refusing to replace an existing test file.' }
New-Item -ItemType Directory -Path $outPath | Out-Null
$env:BUS_SCAN_TRIAL_ROOT=$outPath
$toolchainRoot=& go env GOROOT
if ($LASTEXITCODE -ne 0) { throw 'Cannot locate the installed Go toolchain.' }
$env:PATH=(Join-Path $toolchainRoot 'bin')+';'+$env:PATH
$env:GOTOOLCHAIN='local'
$env:GOPROXY='off'
$env:GOSUMDB='off'
function Run-Checked([string]$Label,[string]$Tree,[string]$Phase,[string]$Test) {
 $env:BUS_SCAN_TRIAL_PHASE=$Phase
 Push-Location $Tree
 try {
  & go test ./internal/bus -run $Test -count=1 -timeout=120s 2>&1 | Tee-Object -FilePath (Join-Path $outPath ($Label+'.log'))
  if ($LASTEXITCODE -ne 0) { throw "$Label exited $LASTEXITCODE" }
 } finally { Pop-Location }
}
$currentTree=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
try {
 Copy-Item -LiteralPath (Join-Path $currentTree 'internal/bus/testdata/scan_yesterday_test.go.txt') -Destination $testPath
 Run-Checked 'old-seed' $oldTree 'seed' '^TestBusScanYesterdayTrial$'
 Run-Checked 'new-migrate' $currentTree 'migrate' $CurrentTest
 Run-Checked 'old-read-deliver-write' $oldTree 'verify' '^TestBusScanYesterdayTrial$'
 Run-Checked 'new-read-back' $currentTree 'verify' $CurrentTest
} finally {
 if (Test-Path -LiteralPath $testPath) { Remove-Item -LiteralPath $testPath }
}
