# Runs the real-program checks listed under "gate" in hidden-checks.json, one
# after another, each on a fresh isolated home. The window never shows: each
# check starts the program hidden and drives its page from headless Edge
# (lib/hidden-desktop.cjs).
#
#   pwsh ./run-hidden.ps1 -Bin <folder> [-Out <folder>] [-Only cold-start-hidden]
#
# -Bin is a folder that holds dinah-desktop.exe, dinah.exe and
# decisions-agent.exe, built from the repository root:
#   go build -tags desktop,production -ldflags "-H windowsgui" -o <bin>/dinah-desktop.exe ./cmd/dinah-desktop
#   go build -o <bin>/dinah.exe ./cmd/dinah
#   go build -ldflags "-H windowsgui" -o <bin>/decisions-agent.exe ./cmd/dinah-desktop/testdata/decisions-agent
#
# The script fails if a check fails, and also if a *-hidden.cjs file is in
# neither list of the manifest, so a new check is run or accounted for.
param(
  [Parameter(Mandatory = $true)][string]$Bin,
  [string]$Out = (Join-Path ([IO.Path]::GetTempPath()) ('dinah-hidden-' + [Guid]::NewGuid().ToString('N'))),
  [string[]]$Only,
  # A check that runs this many seconds is stopped with its process tree.
  [int]$CheckTimeout = 1200
)
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$manifest = Get-Content (Join-Path $here 'hidden-checks.json') -Raw | ConvertFrom-Json
$gate = @($manifest.gate)
$leftOut = @($manifest.leftOut.PSObject.Properties | ForEach-Object Name)
$files = @(Get-ChildItem $here -Filter *-hidden.cjs | ForEach-Object BaseName)
$unlisted = @($files | Where-Object { $gate -notcontains $_ -and $leftOut -notcontains $_ })
$missing = @(($gate + $leftOut) | Where-Object { $files -notcontains $_ })
if ($unlisted) { "Not in hidden-checks.json: $($unlisted -join ', ')"; exit 1 }
if ($missing) { "Listed in hidden-checks.json with no file: $($missing -join ', ')"; exit 1 }
if ($Only) {
  $Only = $Only | ForEach-Object { $_ -split ',' } | ForEach-Object { $_.Trim() } | Where-Object { $_ }
  $unknown = @($Only | Where-Object { $gate -notcontains $_ })
  if ($unknown) { "Not a gate check: $($unknown -join ', ')"; exit 1 }
  $gate = @($gate | Where-Object { $Only -contains $_ })
}
$desktop = Join-Path $Bin 'dinah-desktop.exe'
$agent = Join-Path $Bin 'decisions-agent.exe'
foreach ($program in @($desktop, $agent, (Join-Path $Bin 'dinah.exe'))) {
  if (-not (Test-Path $program)) { "Missing program: $program"; exit 1 }
}
New-Item -ItemType Directory -Force $Out | Out-Null
$node = (Get-Command node).Source
$failed = 0
foreach ($check in $gate) {
  $started = Get-Date
  $psi = [System.Diagnostics.ProcessStartInfo]::new()
  $psi.FileName = $node
  $psi.ArgumentList.Add("$check.cjs")
  $psi.ArgumentList.Add((Resolve-Path $desktop).Path)
  $psi.ArgumentList.Add((Resolve-Path $agent).Path)
  # Each check wants a folder that does not exist yet.
  $psi.ArgumentList.Add((Join-Path $Out $check))
  $psi.WorkingDirectory = $here
  $psi.UseShellExecute = $false
  $psi.CreateNoWindow = $true
  $psi.RedirectStandardOutput = $true
  $psi.RedirectStandardError = $true
  $psi.StandardOutputEncoding = [System.Text.UTF8Encoding]::new($false)
  $psi.StandardErrorEncoding = [System.Text.UTF8Encoding]::new($false)
  $proc = [System.Diagnostics.Process]::Start($psi)
  $stdOut = $proc.StandardOutput.ReadToEndAsync()
  $stdErr = $proc.StandardError.ReadToEndAsync()
  $timedOut = -not $proc.WaitForExit($CheckTimeout * 1000)
  if ($timedOut) { try { $proc.Kill($true) } catch { } }
  $proc.WaitForExit()
  $output = $stdOut.GetAwaiter().GetResult() + "`n" + $stdErr.GetAwaiter().GetResult()
  Set-Content -Path (Join-Path $Out ($check + '.log')) -Value $output -Encoding utf8
  $bad = @($output -split "`n" | Where-Object { $_ -match '^FAIL' })
  $seconds = [Math]::Round(((Get-Date) - $started).TotalSeconds)
  if ($timedOut -or $proc.ExitCode -ne 0 -or $bad.Count) {
    $failed++
    "$check : failed in $seconds s"
    if ($timedOut) { "  the check ran past the $CheckTimeout-second limit and was stopped" }
    $bad | ForEach-Object { "  $($_.TrimEnd())" }
    if (-not $bad.Count) { $output -split "`n" | Where-Object { $_.Trim() } | Select-Object -Last 15 | ForEach-Object { "  $($_.TrimEnd())" } }
  } else { "$check : ok in $seconds s" }
}
"$($gate.Count) hidden checks, $failed failed. Output is in $Out"
if ($failed) { exit 1 }
