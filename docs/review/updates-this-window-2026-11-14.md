# Caller identity and closed Team admission

The branch `frontend/updates-this-window` starts from master `393c763a`. It adds a visible caller label to Updates and preserves the existing closed-chat admission explanation in the Team Report and the lead's Members list. The Architectural Principles remain at `C:\Users\paul\OneDrive\Documents\Architectural Principles.md`.

## Decisions

An Updates running row appends `(this window)` to its PID only when the service supplies `self: true`. That label remains when the installed file is replaced and the caller becomes an older copy. Installed-copy status, release versions and the restart explanation retain their existing meanings. The browser does not infer identity from a path or process name.

Engine confirmed that a closed member waiting for a background place does not acquire a running execution state. The existing Team result gives `life: closed` and the legacy `stateWhy` explanation. The Team Report now uses that explanation in its existing fixed-height reason line. The lead's Members list keeps the Closed chip and exposes the explanation through the row's tooltip and accessible name, and the chip's tooltip and accessible name. Updates to the explanation invalidate the retained row's content signature even when lifecycle and execution data are unchanged.

No CSS, schema or backend field changed. The report's reason line retains ellipsis and its full title and accessible text. The list still shows only a member's name and standard state chip. Generic group membership lists do not receive this Team-only legacy field and were not given an inferred admission state.

## Verification

The following command ran from `cmd/dinah-desktop/frontend/e2e`, with `NODE_PATH` pointing to the existing development dependency tree:

```powershell
./run.ps1 -Only settings-updates-current,settings-updates-installed,teams,team-toolbar,team-navigation -Jobs 1 -Out C:/Users/paul/AppData/Local/Temp/updates-this-window-20261114
```

All five suites passed in 37 seconds, one at a time through the shared FIFO. The harness launches headless Edge. The retained `failures.txt` is empty. The run used synthetic service and Team responses, without a visible window, OS input or live messages.

The new Updates cases verify that only the caller is labelled, that the label survives an older-copy result and that the existing page geometry is retained. The Team cases transition an existing member into closed admission wait and back, verify the full explanation and Closed chip, and compare row rectangles and focus. A longer reason at a 520-pixel viewport retains the same report-row rectangle and its complete accessible explanation. Existing navigation, toolbar and stale-event checks also passed.

Focused ESLint, JavaScript syntax checks and `git diff --check` passed. No Go code changed or Go tests ran. No full browser suite, real-program test, push, merge or installation was performed. The earlier shipped worktrees were not changed.
