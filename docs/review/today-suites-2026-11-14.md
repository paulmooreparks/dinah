# Browser suite audit for 14 November 2026

The audit runs on quality/today-suites from master 4fd079e82b1d1f2d7e5977de85676bdb934f1df5. The inventory includes every top-level JavaScript browser suite added or changed since yesterday's 4fe5a52c, plus the explicitly requested restart suite. There are 45 suites. Standalone hidden-process rehearsal scripts and supporting fixture modules are outside this browser-suite inventory.

Each batch contains at most six suites and runs three at a time through run.ps1. Each batch obtains a new FIFO ticket. The harness launches headless Chromium with fixture host services. No full-suite run, installation or real-home writes are involved.

Evidence is retained under %TEMP%/quality-today-suites-4fd079e8, with an inventory, a log and failures.txt for each batch, and exit codes in results.txt.

## Inventory

- `attention-kept`
- `content-action-outputs`
- `decisions-answers`
- `decisions-attention-entry`
- `decisions-broadcast`
- `decisions-context`
- `decisions-delivery`
- `decisions-drafts`
- `decisions-errors`
- `decisions-list`
- `decisions-narrow`
- `decisions-pages`
- `decisions-races`
- `decisions-reading`
- `decisions-saved-state`
- `decisions-status`
- `help-accuracy-keys`
- `help-accuracy-places`
- `help-accuracy-updates`
- `help-new-work`
- `keyboard`
- `mention-notification`
- `notifications-pane`
- `notifications-popover`
- `restart`
- `restart-left`
- `settings-set-aside-errors`
- `settings-set-aside-list`
- `settings-set-aside-remove`
- `settings-updates-available`
- `settings-updates-current`
- `settings-updates-errors`
- `settings-updates-installed`
- `settings-updates-installing`
- `settings-updates-loading`
- `settings-updates-missing-services`
- `settings-updates-packages`
- `settings-updates-repair`
- `settings-updates-service`
- `settings-updates-unavailable`
- `teams`
- `teams-relay-details`
- `update`
- `whats-new-layout`
- `whats-new-links`


## Results

All 45 suites passed in eight batches. Batch durations were 17, 12, 22, 24, 16, 12, 20 and 22 seconds, excluding FIFO waiting. Every batch exited zero and every failures.txt was empty. No suite or product changes were required. These results apply to the frozen base above, before later answer-origin changes.


## Diagram follow-up

The follow-up uses master d6e00dea82ce9846647f76a3f8831247199099aa, with the audit record rebased onto it. The lead reported three earlier timeouts at the Mermaid expand button, including an isolated run and a pre-package-update worktree. Those reports are retained as failures under load; their cause is unconfirmed.

The unchanged diagram suite passed four isolated runs, each in 8 seconds. The matching five-suite batch (contained-content, content-host-location, content-action-outputs, diagrams and document-diagrams) also passed in 14 seconds. No first bad merge can be identified from these results. The lead explicitly directed a condition-based wait and fuller failure evidence if isolated and batched runs passed.

The diagram suite now waits for the Mermaid button to be attached and enabled, its SVG to exist, and the D2 result and error to be visible before making its existing assertions. The fixed three-second render delay is removed. No assertion was weakened and no timeout was increased. The product is unchanged.

The browser runner now saves each suite's complete stdout and stderr to a named .log file in the run's output directory. Its console summary and failing exit remain unchanged. A new PowerShell test runs an invented failing Node fixture through the real runner and checks that the failed exit, earlier passing assertion, timeout and indented call log all survive. This fixture does not launch a browser.

Baseline evidence is under %TEMP%/quality-diagrams-master, quality-diagrams-repeat-1 through quality-diagrams-repeat-3, and quality-diagrams-baseline-batch. The final five-suite batch passed in 9 seconds under quality-diagrams-condition-batch, with exit zero and an empty failures.txt. The runner evidence test passed; its invented child failure correctly exited one and the parent test verified the retained output. Its evidence is in %TEMP%/dinah-run-output-2accbd2a249c43a1bc4a8ae1a97bc044/output. JavaScript syntax and git diff --check passed. No Go suite, full browser suite, installation, push, master merge, real-home write or native input was performed.

