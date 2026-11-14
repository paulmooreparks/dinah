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

