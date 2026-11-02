# Runs every guard the November 2026 review added (docs/design/invariants.md),
# as listed in scripts/guards.json, and fails if any listed Go test is
# missing, skipped or failing, or any listed browser suite fails. The same
# command runs locally and in CI.
#
#   pwsh scripts/guards.ps1                      # everything, before a push
#   pwsh scripts/guards.ps1 -Skip browser        # the Go guards only
#
# The forbidden-terms list is private and lives outside the repository, so
# that guard runs only where DINAH_FORBIDDEN_TERMS names it. Locally it is
# required. CI, which has no copy of the list, passes -WithoutForbiddenTerms,
# and the guard must then report itself skipped, never silently pass.
param(
  [ValidateSet('go', 'browser')][string[]]$Skip = @(),
  [switch]$WithoutForbiddenTerms,
  [string]$Out = (Join-Path ([IO.Path]::GetTempPath()) 'dinah-guards')
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot
$manifest = Get-Content (Join-Path $PSScriptRoot 'guards.json') -Raw | ConvertFrom-Json
$Out = [IO.Path]::GetFullPath($Out)
New-Item -ItemType Directory -Force $Out | Out-Null
$failures = [Collections.Generic.List[string]]::new()
$forbidden = 'TestNoForbiddenTermsInTrackedFiles'

if (-not $WithoutForbiddenTerms -and -not $env:DINAH_FORBIDDEN_TERMS) {
  [Console]::Error.WriteLine('DINAH_FORBIDDEN_TERMS is not set. Set it to the private list before pushing, or pass -WithoutForbiddenTerms where the list is unavailable (CI).')
  exit 2
}
if ($WithoutForbiddenTerms) { Remove-Item Env:DINAH_FORBIDDEN_TERMS -ErrorAction SilentlyContinue }

Push-Location $repo
try {
  $goos = (& go env GOOS).Trim()
  if ($Skip -notcontains 'go') {
    foreach ($entry in $manifest.go) {
      $names = @($entry.tests)
      $pattern = '^(' + ($names -join '|') + ')$'
      $label = ($entry.package -replace '[^A-Za-z0-9]+', '_').Trim('_')
      if (-not $label) { $label = 'root' }
      $report = Join-Path $Out ($label + '.jsonl')
      & pwsh -NoProfile (Join-Path $PSScriptRoot 'go-gate.ps1') $entry.package -run $pattern -count=1 -json > $report
      $exit = $LASTEXITCODE
      $outcome = @{}
      foreach ($line in Get-Content $report) {
        if (-not $line.StartsWith('{')) { continue }
        $evt = $line | ConvertFrom-Json
        if ($evt.Test -and $evt.Test -notmatch '/' -and $evt.Action -in @('pass', 'fail', 'skip')) { $outcome[$evt.Test] = $evt.Action }
      }
      foreach ($name in $names) {
        $got = $outcome[$name]
        $platforms = if ($entry.onlyOn) { $entry.onlyOn.$name } else { $null }
        if ($platforms -and $platforms -notcontains $goos) {
          # A guard for another operating system skips here, or is not compiled here at all.
          if ($got -and $got -ne 'skip') { $failures.Add("$($entry.package) $name`: runs only on $($platforms -join ', '), but got $got on $goos") }
          continue
        }
        $want = if ($WithoutForbiddenTerms -and $name -eq $forbidden) { 'skip' } else { 'pass' }
        if ($got -ne $want) { $failures.Add("$($entry.package) $name`: expected $want, got $(if ($got) { $got } else { 'no result' })") }
      }
      if ($exit -ne 0 -and -not ($failures | Where-Object { $_.StartsWith($entry.package + ' ') })) {
        $failures.Add("$($entry.package): go test exited $exit; see $report")
      }
    }
  }
  if ($Skip -notcontains 'browser') {
    $e2e = Join-Path $repo 'cmd/dinah-desktop/frontend/e2e'
    & pwsh -NoProfile (Join-Path $e2e 'run.ps1') -Only (@($manifest.browser) -join ',') -Out (Join-Path $Out 'browser')
    if ($LASTEXITCODE -ne 0) { $failures.Add("browser guard suites failed; see $(Join-Path $Out 'browser')") }
  }
} finally {
  Pop-Location
}

if ($failures.Count) {
  'Guards failed:'
  $failures | ForEach-Object { '  ' + $_ }
  exit 1
}
'All guards passed.'
