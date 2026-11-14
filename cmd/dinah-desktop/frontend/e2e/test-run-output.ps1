# Verify the runner's failure evidence using invented output, without a browser.
$ErrorActionPreference = 'Stop'
$root = Join-Path ([IO.Path]::GetTempPath()) ('dinah-run-output-' + [guid]::NewGuid().ToString('N'))
$suite = Join-Path $root 'e2e'
New-Item -ItemType Directory -Path (Join-Path $suite 'lib') -Force | Out-Null
Set-Content (Join-Path $root 'index.html') '<!doctype html><title>Fixture</title>'
Copy-Item (Join-Path $PSScriptRoot 'run.ps1') $suite
Copy-Item (Join-Path $PSScriptRoot 'lib/browser-lock.ps1') (Join-Path $suite 'lib')
@'
console.log('PASS earlier assertion');
console.error('locator.click: Timeout 10000ms exceeded.');
console.error('Call log:');
console.error('  - waiting for the invented button');
console.error('  - an invented overlay intercepts pointer events');
process.exitCode = 1;
'@ | Set-Content (Join-Path $suite 'failure.js')
$out = Join-Path $root 'output'
& pwsh -NoProfile -File (Join-Path $suite 'run.ps1') -Only failure -Out $out
if ($LASTEXITCODE -ne 1) { throw 'The runner did not reject the failed fixture.' }
$full = Get-Content (Join-Path $out 'failure.log') -Raw
foreach ($line in @('PASS earlier assertion', 'locator.click: Timeout 10000ms exceeded.', 'Call log:', '  - waiting for the invented button', '  - an invented overlay intercepts pointer events')) {
  if (-not $full.Contains($line)) { throw "The full suite log lost: $line" }
}
if (-not (Get-Content (Join-Path $out 'failures.txt') -Raw).Contains('overlay intercepts pointer events')) {
  throw 'The failure summary lost the actionability reason.'
}
Write-Output "PASS failed exit, prior output and complete call log retained in $out"
