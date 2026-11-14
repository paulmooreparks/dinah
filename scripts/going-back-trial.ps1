param(
 [Parameter(Mandatory=$true)][string]$YesterdayTree,
 [Parameter(Mandatory=$true)][string]$Out
)
$ErrorActionPreference='Stop'
$outPath=[IO.Path]::GetFullPath($Out)
$tempRoot=[IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')+'\'
if (-not $outPath.StartsWith($tempRoot,[StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath $outPath)) {throw 'Supply a new child folder of TEMP'}
$oldTree=[IO.Path]::GetFullPath($YesterdayTree)
$expected=& git -C $oldTree rev-parse HEAD
if ($LASTEXITCODE -ne 0 -or -not $expected.StartsWith('4fe5a52c')) {throw 'YesterdayTree must be the separate 4fe5a52c checkout'}
$testPath=Join-Path $oldTree 'cmd/dinah-desktop/going_back_external_test.go'
if (Test-Path -LiteralPath $testPath) {throw 'Refusing to replace an existing test file'}
New-Item -ItemType Directory -Path $outPath | Out-Null
$env:DINAH_GOING_BACK_ROOT=$outPath
$env:DINAH_HOME=Join-Path $outPath 'unused-home'
$env:GKC_HOME=Join-Path $outPath 'unused-other-home'
$toolchainRoot=& go env GOROOT
if ($LASTEXITCODE -ne 0) {throw 'Cannot locate the installed Go toolchain'}
$env:PATH=(Join-Path $toolchainRoot 'bin')+';'+$env:PATH
$env:GOTOOLCHAIN='local'
$env:GOPROXY='off'
$env:GOSUMDB='off'
function Run-Checked([string]$Label,[scriptblock]$Command) {
 & $Command 2>&1 | Tee-Object -FilePath (Join-Path $outPath ($Label+'.log'))
 if ($LASTEXITCODE -ne 0) {throw "$Label exited $LASTEXITCODE"}
}
try {
 Run-Checked 'new-writes' {go test ./cmd/dinah-desktop -run '^TestGoingBackWriteCurrentState$' -count=1 -v}
 Copy-Item -LiteralPath (Join-Path $PSScriptRoot '../cmd/dinah-desktop/testdata/going-back/yesterday_test.go.txt') -Destination $testPath
 Push-Location $oldTree
 try {
  Run-Checked 'old-reads-writes' {go test ./cmd/dinah-desktop -run '^TestGoingBackYesterday$' -count=1 -v}
  Run-Checked 'old-build' {go build -buildvcs=false -tags desktop,production -ldflags '-H windowsgui -X main.appVersion=rollback-old' -o (Join-Path $outPath 'yesterday.exe') ./cmd/dinah-desktop}
 } finally {Pop-Location}
 Run-Checked 'new-reads-back' {go test ./cmd/dinah-desktop -run '^TestGoingBackCurrentReadsAfterYesterday$' -count=1 -v}
} finally {
 # This exact file was absent before this run and contains only our fixture.
 if (Test-Path -LiteralPath $testPath) {Remove-Item -LiteralPath $testPath}
}
