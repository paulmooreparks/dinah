param(
  [ValidateRange(1,20)][int]$Runs = 20,
  [ValidateRange(1,20)][int]$StartRun = 1,
  [switch]$ArrivingOnly,
  [string]$OnlySuite,
  [string]$StopOnSuite = 'chat-arriving',
  [string]$FailurePattern = '.',
  [string]$Out = (Join-Path ([IO.Path]::GetTempPath()) 'chat-arriving-combined')
)
$ErrorActionPreference = 'Stop'
if ($StartRun -gt $Runs) { throw 'StartRun must not exceed Runs.' }
$root = Split-Path $PSScriptRoot
$runner = Join-Path $root 'cmd/dinah-desktop/frontend/e2e/run.ps1'
$suites = @('chat-arriving','chat-replay-isolation','chat-saved-restore',
  'decisions-answer-card','decisions-answer-layout','decisions-answers',
  'decisions-attention-entry','decisions-broadcast','decisions-context',
  'decisions-delivery','decisions-drafts','decisions-errors','decisions-list',
  'decisions-narrow','decisions-pages','decisions-races','decisions-reading',
  'decisions-saved-state','decisions-status','delivered-messages','saved-draw-once')
if ($ArrivingOnly) { $suites = @('chat-arriving') }
if ($OnlySuite) { $suites = @($OnlySuite) }
$label = if ($ArrivingOnly -or $OnlySuite) { 'Targeted' } else { 'Combined' }
New-Item -ItemType Directory -Force $Out | Out-Null
$records = @()
if ($StartRun -gt 1 -and (Test-Path (Join-Path $Out 'runs.json'))) {
  $records = @(Get-Content -LiteralPath (Join-Path $Out 'runs.json') -Raw | ConvertFrom-Json)
}
for ($i = $StartRun; $i -le $Runs; $i++) {
  $folder = Join-Path $Out ('run-{0:d2}' -f $i)
  $started = Get-Date
  $output = & pwsh -NoProfile -File $runner -Only ($suites -join ',') -Jobs 1 -Out $folder 2>&1
  $code = $LASTEXITCODE
  $output | Set-Content -LiteralPath (Join-Path $folder 'console.txt') -Encoding utf8
  $records += [pscustomobject]@{ run=$i; exit=$code; seconds=((Get-Date)-$started).TotalSeconds; folder=$folder }
  $records | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $Out 'runs.json') -Encoding utf8
  Write-Output "$label run $i finished with exit $code. Evidence: $folder"
  if ($code -ne 0) { $output | Write-Output }
  if ((Get-Content -LiteralPath (Join-Path $folder 'failures.txt') | Select-String ('^' + [regex]::Escape($StopOnSuite) + ' :')) -and
      (Get-Content -LiteralPath (Join-Path $folder ($StopOnSuite + '.log')) | Select-String $FailurePattern)) { exit 1 }
}
if ($records | Where-Object { $_.exit -ne 0 }) { exit 1 }
