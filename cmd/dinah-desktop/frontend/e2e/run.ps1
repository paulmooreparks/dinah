# Most browser suites use a stubbed desktop window. web-skins builds an isolated Go test host.
# Each suite prints PASS or FAIL lines; this script prints only the failures and
# exits 1 if there were any. Screenshots go to $Out.
#
#   cd cmd/dinah-desktop/frontend/e2e; npm install; pwsh ./run.ps1
#   pwsh ./run.ps1 -Only editor,files
#   pwsh ./run.ps1 -Shard 3/8
#
# -Shard i/n runs the i-th of n shares of the suites, so that n runners can
# divide one run between them. The shares are balanced by the seconds each
# suite took in a recorded run (suite-times.json), and every suite belongs
# to exactly one share. The suites start longest first, so a long suite never
# starts last and holds the run alone. Each run writes the seconds its suites
# took to suite-times.json in $Out; copy a full run's file over the recorded
# one when the suites have changed enough to unbalance the shares.
#
# set-aside.json lists the checks that are set aside under the merge guide's
# rule for a red build: each has its suite, a regular expression for the
# check's name, the date, the owner who will repair it, and the cause. A suite
# whose only failures are set-aside checks is reported as ok, and every run
# prints the list, so a set-aside check stays in view until its entry is
# removed. A set-aside check that passes is not reported.
#
# Each suite runs its own browser and shares nothing with the others, so
# they run side by side, -Jobs at a time, a quarter of the processors by
# default. Half the processors took about 63 s on Paul's 32-processor
# machine but failed a suite in 4 of 7 runs while the machine did other
# work; a quarter takes about 70 s and passed 5 runs of 5 under the same
# load (26 October 2026). The results are printed in the suites' order,
# with the time the run took.
param(
  [string[]]$Only,
  [string]$Shard,
  # -List prints the suites the run would start, in order, and runs nothing.
  [switch]$List,
  [string]$Out = (Join-Path ([IO.Path]::GetTempPath()) 'dinah-e2e'),
  [int]$Jobs = [Math]::Max(2, [Math]::Floor([Environment]::ProcessorCount / 4)),
  # A suite that runs this many seconds is killed with its whole process tree
  # and reported as failed: waiting on node had no limit before, and a wedged
  # ui-state held a run for over twenty minutes overnight (30 October 2026).
  [int]$SuiteTimeout = 600
)
$ErrorActionPreference = 'Stop'
$here = $PSScriptRoot
$page = 'file:///' + ((Resolve-Path (Join-Path $here '..\index.html')).Path -replace '\\', '/')
New-Item -ItemType Directory -Force $Out | Out-Null
$suites = Get-ChildItem $here -Filter *.js | ForEach-Object BaseName
# -Only a,b from another shell arrives as the one string "a,b", so the names
# are split on commas here; a name that matches no suite is an error, never
# a run of nothing that reports success.
if ($Only) {
  $Only = $Only | ForEach-Object { $_ -split ',' } | ForEach-Object { $_.Trim() } | Where-Object { $_ }
  $unknown = $Only | Where-Object { $suites -notcontains $_ }
  if ($unknown) { "No such suite: $($unknown -join ', ')"; exit 1 }
  $suites = $suites | Where-Object { $Only -contains $_ }
}
# A suite with no recorded time is given the median, so a new suite neither
# empties a share nor overloads one.
$recorded = @{}
$timesFile = Join-Path $here 'suite-times.json'
if (Test-Path $timesFile) {
  (Get-Content $timesFile -Raw | ConvertFrom-Json).PSObject.Properties | ForEach-Object { $recorded[$_.Name] = [double]$_.Value }
}
$known = @($recorded.Values | Sort-Object)
$median = if ($known.Count) { $known[[Math]::Floor($known.Count / 2)] } else { 1 }
$weight = { param($name) if ($recorded.ContainsKey($name)) { $recorded[$name] } else { $median } }
# Longest first, and by name among equals, so the order is the same everywhere.
$suites = @($suites | Sort-Object @{ Expression = { & $weight $_ }; Descending = $true }, @{ Expression = { $_ } })
if ($Shard) {
  if ($Shard -notmatch '^(\d+)/(\d+)$' -or [int]$Matches[1] -lt 1 -or [int]$Matches[1] -gt [int]$Matches[2]) { "-Shard takes i/n with 1 <= i <= n, not $Shard"; exit 1 }
  $share = [int]$Matches[1]; $shares = [int]$Matches[2]
  # Each suite, longest first, goes to the share with the least time so far.
  $totals = [double[]]::new($shares)
  $mine = [System.Collections.Generic.List[string]]::new()
  foreach ($s in $suites) {
    $least = 0
    for ($i = 1; $i -lt $shares; $i++) { if ($totals[$i] -lt $totals[$least]) { $least = $i } }
    $totals[$least] += & $weight $s
    if ($least -eq $share - 1) { $mine.Add($s) }
  }
  $suites = @($mine)
  "Share $share of $shares : $($suites.Count) suites, about $([Math]::Round($totals[$share - 1])) recorded seconds"
}
if (-not @($suites).Count) { 'No suites to run'; exit 1 }
if ($List) { $suites; exit 0 }
. (Join-Path $here 'lib/browser-lock.ps1')
$browserLease = $null
$browserTicket = $env:DINAH_BROWSER_TEST_TICKET
if (-not (Test-DinahBrowserTicket (Get-DinahBrowserLockPath) $browserTicket)) {
  $browserLease = Enter-DinahBrowserLock (Get-DinahBrowserLockPath) ('run.ps1 in ' + $here)
  $browserTicket = $browserLease.Ticket
}
try {
$started = Get-Date
# failures.txt belongs to this run alone: it is truncated now, so an install
# check can never read last night's (30 October 2026), and written again at
# the end with what the suites actually printed.
Set-Content -Path (Join-Path $Out 'failures.txt') -Value @() -Encoding utf8
$node = (Get-Command node).Source
$results = $suites | ForEach-Object -ThrottleLimit $Jobs -Parallel {
  $s = $_
  # Each suite is a process of its own, so one that wedges can be stopped
  # without taking the run with it.
  $psi = [System.Diagnostics.ProcessStartInfo]::new()
  $psi.FileName = $using:node
  $psi.ArgumentList.Add("$s.js")
  $psi.ArgumentList.Add($using:page)
  $psi.ArgumentList.Add($using:Out)
  $psi.WorkingDirectory = $using:here
  $psi.Environment['DINAH_BROWSER_TEST_TICKET'] = $using:browserTicket
  $psi.UseShellExecute = $false
  $psi.CreateNoWindow = $true
  $psi.RedirectStandardOutput = $true
  $psi.RedirectStandardError = $true
  # Node writes UTF-8; the console code page must not decode its output.
  $psi.StandardOutputEncoding = [System.Text.UTF8Encoding]::new($false)
  $psi.StandardErrorEncoding = [System.Text.UTF8Encoding]::new($false)
  $clock = [System.Diagnostics.Stopwatch]::StartNew()
  $proc = [System.Diagnostics.Process]::Start($psi)
  $stdOut = $proc.StandardOutput.ReadToEndAsync()
  $stdErr = $proc.StandardError.ReadToEndAsync()
  $timedOut = -not $proc.WaitForExit($using:SuiteTimeout * 1000)
  if ($timedOut) {
    try { $proc.Kill($true) } catch { }
  }
  # The plain wait lets the reads above finish before their results are taken.
  $proc.WaitForExit()
  $result = ($stdOut.GetAwaiter().GetResult() + "`n" + $stdErr.GetAwaiter().GetResult())
  if ($timedOut) { $result = "FAIL the suite ran past the $using:SuiteTimeout-second limit and was killed; its last lines are below`n" + $result }
  $bad = @($result -split "`n" | Where-Object { $_ -match '^FAIL|Error:|Timeout.*exceeded|Timed out after|^    at .*\.js:' })
  [pscustomobject]@{ Suite = $s; Seconds = [Math]::Round($clock.Elapsed.TotalSeconds, 1); Failed = ($bad.Count -gt 0 -or $proc.ExitCode -ne 0); Lines = $bad; Output = $result }
}
$setAside = @()
$setAsideFile = Join-Path $here 'set-aside.json'
if (Test-Path $setAsideFile) { $setAside = @((Get-Content $setAsideFile -Raw | ConvertFrom-Json).checks) }
$failed = 0
# failures.txt in $Out keeps what a failing suite printed, apart from its PASS lines,
# so a failure seen once is not lost when the screen is closed. It is empty when
# every suite passes.
$record = [System.Collections.Generic.List[string]]::new()
foreach ($r in ($results | Sort-Object Suite)) {
  # Keep stdout and stderr in full, including Playwright's indented call log
  # and assertions that passed before a failure. Console output stays brief.
  Set-Content -Path (Join-Path $Out ($r.Suite + '.log')) -Value $r.Output -Encoding utf8
  # A suite fails with exit code 1 when a check fails. When every line that
  # counts against it is a set-aside check's FAIL line, the suite is ok.
  $excused = @()
  $mine = @($setAside | Where-Object { $_.suite -eq $r.Suite })
  if ($r.Failed -and $mine.Count -and $r.Lines.Count) {
    $excused = @($r.Lines | Where-Object { $line = $_; $line -match '^FAIL ' -and @($mine | Where-Object { ($line.Substring(5) -split ' :: ', 2)[0].TrimEnd() -match $_.check }).Count })
    if ($excused.Count -eq $r.Lines.Count) { $r.Failed = $false }
  }
  if ($r.Failed) {
    $failed++; "$($r.Suite) :"; $r.Lines | ForEach-Object { "  $_" }
    $record.Add("$($r.Suite) :")
    $r.Output -split "`n" | Where-Object { $_.Trim() -and $_ -notmatch '^PASS ' } | ForEach-Object { $record.Add("  $($_.TrimEnd())") }
  } elseif ($excused.Count) {
    "$($r.Suite) : ok apart from $($excused.Count) set-aside check(s)"; $excused | ForEach-Object { "  set aside: $_" }
  } else { "$($r.Suite) : ok" }
}
foreach ($entry in ($setAside | Where-Object { $suites -contains $_.suite })) { "Set aside since $($entry.date), owner $($entry.owner): $($entry.suite), $($entry.check)" }
Set-Content -Path (Join-Path $Out 'failures.txt') -Value $record -Encoding utf8
$took = [ordered]@{}
foreach ($r in ($results | Sort-Object Suite)) { $took[$r.Suite] = $r.Seconds }
Set-Content -Path (Join-Path $Out 'suite-times.json') -Value ($took | ConvertTo-Json) -Encoding utf8
"{0} suites in {1:n0} s, {2} at a time" -f @($suites).Count, ((Get-Date) - $started).TotalSeconds, $Jobs
if ($failed) { "$failed suite(s) failed"; exit 1 }

} finally { if ($browserLease) { $browserLease.Stream.Dispose() } }
