# Background admission limit and waiting-sender notices

The implementation is commit 6f357db0518e403a64a2b16ae4eb608e61a7a226 on engine/background-limit, based on master 393c763a in .worktrees/engine-background-limit. It changes the saved application limit to a configurable default of eight and privately notifies the senders of messages whose target chat waits for a background place. Settings and DESIGN.md state the behavior. No merge, push or installation has occurred.

The limit accepts one through sixty-four. Settings changes are persisted through updateSettings and applied under the runtime mutex without restarting. The next wake round reads the setting under that same mutex. Lowering it preserves running chats and pending connections. Admission waits until places are available. Startup-warmed chats retain their separate allowance.

The notice follows admission and idle yielding. It identifies the chat using the bus display title and states that every background place is in use. Original messages stay queued. It reaches each original agent sender privately, or the person's bus inbox for a message from the person. Repeated checks and later details from an already-notified sender do not repeat it during the wait. Durable host-result keys deduplicate retried deliveries and restart reconciliation. A host result cannot itself trigger a waiting-sender notice, and a subsequent yield does not send another notice. The receipt written at send time is unchanged.

## Verification

The complete package command was run through scripts/go-gate.ps1 and the shared FIFO:

```
go test ./internal/bus/... ./internal/bushost/... ./internal/sessionhost/... ./cmd/dinah-desktop/... -count=1
```

The final run passed on implementation commit 6f357db0. The reported package times were bus 68.212 seconds, bushost 13.758 seconds, sessionhost 23.943 seconds and desktop 263.226 seconds. The desktop product subpackage reported no test files. The receipt is retained locally at %TEMP%/dinah-background-limit-packages-2.log.

Two initial focused runs failed in the new notice fixture because it attempted a direct send through the live-chat address resolver after closing the target. The fixture now sends a group assignment to the closed member, matching the incident. The third focused run passed. The first complete run then found an existing recovery fixture's hard-coded expectation of three admissions. It now explicitly selects a three-place setting, preserving that fixture's original bound and recovery assertions. The complete corrected run above passed. Earlier failures remain in %TEMP%/dinah-background-limit-focused.log, focused-2.log and packages.log with the same dinah-background-limit prefix.

Node syntax checks passed for frontend/app.js and frontend/settings-applet.js. The added-line punctuation check passed. These checks used the same FIFO and opened no browser.

Focused race checks passed for the same four package trees, using -race with TestBackgroundLimit, TestWakeWaiting, TestWaitingSender and TestRuntimeAdmissionCompletionAndWindowsShareStateAuthority. Reported times were 3.795, 1.813, 1.184 and 10.325 seconds respectively. The command exited zero. The Windows linker also printed ".rsrc merge failure: multiple non-default manifests" while linking the desktop test binary; the binary then ran and Go reported that package as passing. This diagnostic is retained rather than described as a clean link. No application build was verified. The receipt is %TEMP%/dinah-background-limit-extra.log.

## Limits

No rendered Settings check or full browser suite was run. No desktop automation, operating-system input, visible test window, push, merge or install was performed. Verification used temporary test state and fake provider connections. The Team view's existing closed state and stateWhy contract is preserved; only its displayed limit is made configurable. Front End owns its separate Team page/list wording check.
