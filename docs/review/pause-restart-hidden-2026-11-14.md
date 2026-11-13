# Pause and Restart verification

Quality checked master `29a02bc17daba12671518c87a86b0dc81c558f60` on branch `quality/pause-restart-check`. The work used an isolated temporary home, `DINAH_SHELL_ENTRY_FILE`, a hidden Wails window and headless Chromium. Every build and test went through the shared FIFO. No installed product was changed and no desktop input was sent.

## Correction

Cancel Restart could consume the pause record while steering was still being submitted. The record did not yet mark the chat as told, so cancellation discarded it without sending a carry-on message. The regression failed with `cancel lost the carry-on message during steering` before the correction.

Cancellation now lets the submitting pause call finish the record and deliver cancellation. The pause call marks submission in progress before starting the idle watcher, and that watcher waits for submission to settle before automatic closure. The correction preserves the existing saved-history recovery rules.

## Evidence

The final focused Go command was:

```powershell
scripts/with-test-lock.ps1 go test ./cmd/dinah-desktop -run 'Test(Pause|AChatIsTold|WaitingToRestart|ANewerInstalledBuild|ANewBuildWaits)' -count=1 -timeout=60s -v
```

All nine tests passed in 4.937 seconds. Three new tests exercise actual TurnIO submission and the durable bus with the existing in-process test provider. They verify steering ends a working turn, cancellation wakes it, cancellation during submission does not lose it, and a fresh App receives the paused-chat continuation once. Replaying the saved restart record after successful enqueue does not produce a second prompt.

The hidden executable tests use `frontend/e2e/restart-hidden.cjs` and the harmless ACP stand-in in `testdata/restart-agent/main.go`. The stand-in neither contacts a model nor executes tools. It advertises session loading but has no steering hook. Its initial completed turn makes the session eligible for native recovery. A second turn stays open until a fixture file is created or the old process is stopped.

| Scenario | Observed result |
| --- | --- |
| Unsteerable chat, Restart Now | The old process waited with one working chat. Restart Now and the page exit confirmation closed it. A replacement served a new entry address, reopened the session and submitted one prompt saying its turn was cut short. |
| Unsteerable chat, natural completion | The old process waited until the fixture completed its turn, then exited automatically. The replacement submitted one prompt saying the last turn ended before restart. |
| Steerable chat, fresh App | The Go test provider accepted the pause request and ended the turn. The persisted record had Told and Ended set. A fresh App reopened the session and submitted one paused-chat continuation. This did not cross a native process boundary. |
| Cancel after pause or during submission | Both Go tests observed a cancellation continuation starting a turn. Replaying the durable record did not duplicate the already-delivered continuation. |

The candidate executable was built with `-tags desktop,production -ldflags '-H windowsgui'`; its SHA-256 is `9a6d0b7e84d9d728aacb1ec16bed34f494a335cae5f99f3f222e88a429ad67a9`. The harness places it only under its temporary home, renames that temporary executable and appends a fixture marker to an identical replacement. This exercises update detection and process replacement without installing a release. The launcher uses `windowsHide:true`; Chromium uses `headless:true`. Cleanup targets only the fixture executable's absolute paths.

Evidence is under `%TEMP%/dinah-pause-restart-check`. The `candidate-now` and `candidate-finish` directories contain passing receipts and isolated application logs. To reproduce, build the desktop and stand-in into a temporary folder, set `NODE_PATH` to an existing Playwright installation, and run:

```powershell
scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/restart-hidden.cjs <desktop.exe> <restart-agent.exe> <new-output-folder>
```

Set `RESTART_FINISH=1` for natural completion. Without it, the harness chooses Restart Now. The final stand-in records only the first prompt block, excluding the appended private applet context.

## Limits and retained failures

An interrupted first-ever turn did not resume. In `run2`, both native processes started correctly, but the bus logged `The first turn did not finish; continue from saved history to reopen this chat.` The continuation remained waiting. `native_sessions.go` intentionally refuses this unverified native session. Removing that guard or introducing automatic saved-history recovery needs a separate decision. Set `RESTART_FIRST_TURN=1` to reproduce this boundary; that mode is expected to fail the resume assertion.

The complete steerable-agent sequence has not been shown across a real native restart. Steering and cancellation were verified with the in-process Go provider; the native restart used an unsteerable ACP stand-in. A model's compliance with the pause instruction is also unverified. The tests establish host delivery and turn transitions, not whether a real model chooses a safe stopping point.

The focused race command printed passing test bodies but exited 1 with `.rsrc merge failure: multiple non-default manifests`. It is not a clean race result. No full browser suite ran. Early harness development also retained a missing Playwright import, a missing workspace selection, and a test timeout caused by waiting for a turn before releasing its steering submission. Those were harness defects, not successful product checks.

The hidden tests observed one delivery plus a subsequent delivery interval. The separate durable-record replay assertion tests idempotency directly; neither is a proof against every possible crash or filesystem failure. The old executable's exit and the replacement's new authenticated entry address establish process replacement. The hidden native window was not visually inspected.

The branch is ready for lead review of the cancellation correction. The first-turn recovery boundary and the missing real-process steering check remain explicit acceptance limits.
