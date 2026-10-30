# Runs the browser suites against the page in a stubbed window (no Go, no Devin).
# Each suite prints PASS or FAIL lines; this script prints only the failures and
# exits 1 if there were any. Screenshots go to $Out.
#
#   cd cmd/dinah-desktop/frontend/e2e; npm install; pwsh ./run.ps1
#   pwsh ./run.ps1 -Only editor,files
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
. (Join-Path $here 'lib/browser-lock.ps1')
$browserLease = Enter-DinahBrowserLock (Get-DinahBrowserLockPath) ('run.ps1 in ' + $here)
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
  $psi.Environment['DINAH_BROWSER_TEST_TICKET'] = $using:browserLease.Ticket
  $psi.UseShellExecute = $false
  $psi.RedirectStandardOutput = $true
  $psi.RedirectStandardError = $true
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
  [pscustomobject]@{ Suite = $s; Failed = ($bad.Count -gt 0 -or $proc.ExitCode -ne 0); Lines = $bad; Output = $result }
}
$failed = 0
# failures.txt in $Out keeps what a failing suite printed, apart from its PASS lines,
# so a failure seen once is not lost when the screen is closed. It is empty when
# every suite passes.
$record = [System.Collections.Generic.List[string]]::new()
foreach ($r in ($results | Sort-Object Suite)) {
  if ($r.Failed) {
    $failed++; "$($r.Suite) :"; $r.Lines | ForEach-Object { "  $_" }
    $record.Add("$($r.Suite) :")
    $r.Output -split "`n" | Where-Object { $_.Trim() -and $_ -notmatch '^PASS ' } | ForEach-Object { $record.Add("  $($_.TrimEnd())") }
  } else { "$($r.Suite) : ok" }
}
Set-Content -Path (Join-Path $Out 'failures.txt') -Value $record -Encoding utf8
"{0} suites in {1:n0} s, {2} at a time" -f @($suites).Count, ((Get-Date) - $started).TotalSeconds, $Jobs
if ($failed) { "$failed suite(s) failed"; exit 1 }

} finally { $browserLease.Stream.Dispose() }
