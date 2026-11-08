# Dinah external development handoff, 9 November 2026

## Current status after commit, merge and installation

On 9 November 2026, commit 8315d4f integrated PUDL 0.105.0, the chat restoration and provider shutdown fixes, regression tests and this handoff. It was fast-forwarded into master in C:/Users/paul/source/repos/studio. The outstanding source changes described below are now committed. The earlier sections remain as historical investigation context.

Build 0.0.0-pudl-01050-20261109 was installed at 00:38 Singapore. Its SHA256 is A5C0FE1489E0CAB1EDAEDFA14564D060A0F92D0FAD1556B8E46409D5A0F3F6F8. The rollback executable is C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-pudl-01050-20261109-003840. The receipt is in the install worktree at build/pudl-01050-release/install-receipt.json. Running process IDs remained unchanged; the desktop was not restarted. Native Restart validation remains outstanding.

All six launcher/taskbar suites passed with 0.105.0. All seven replay suites also passed against the new distribution before merge. The production build passed. Private transcripts, build artifacts and unrelated main-worktree files were excluded from the commit. Nothing was pushed.

The outside agent can now begin from master in the main studio directory. Private evidence remains in C:/Users/paul/source/repos/dinah-install-20261105/build/. The preserved cleanup/history-capture stash remains untouched.


## Start here

Paul is moving development to an agent outside Dinah so debugging, testing and desktop restarts do not interrupt the development agent. Continue from the existing install worktree. Do not start from a clean checkout of HEAD: the installed replay and shutdown fixes are still uncommitted.

The immediate remaining validation is a native Restart command from the newly loaded build, followed by checking transcript stability. Paul reported that manual shutdown did not hang, startup was clean, and the PUDL Team Lead transcript looked correct on superficial inspection after loading the latest build. That report is encouraging but does not establish that the Restart command works or that every history ordering case is fixed.

This handoff was written after checking local Git status, the installed executable hash, the bundled PUDL version and retained test records. No tests, installation or restart were performed while writing it.

## Checkout and installed identity

| Item | Current value |
| --- | --- |
| Main repository location | `C:/Users/paul/source/repos/studio` |
| Worktree containing this work | `C:/Users/paul/source/repos/dinah-install-20261105` |
| Branch | `feature/pudl-pinned-section-candidate` |
| HEAD | `8586abe999e9a0d63b3d065cf2e46c109063cdeb` |
| Previous integrated baseline | `902df02`, Integrate installed desktop launcher, taskbar and reliability updates |
| Installed executable | `C:/Users/paul/.dinah/bin/dinah-desktop.exe` |
| Installed build version | `0.0.0-background-replay-20261108` |
| Installed SHA256, rechecked 9 November | `9B99E2E0194B9E249C7D75518B868D0ACECCA647BC711D3F878B1660A4DBEC86` |
| Build executable | `build/restart-replay-20261108-0521/dinah-desktop.exe` |
| Latest installation | 8 November 2026, 05:43 Singapore |
| Rollback executable | `C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-background-replay-20261108-054300` |
| Receipt | `build/restart-replay-20261108-0521/install-receipt.json` |

Paths below are relative to the install worktree unless explicitly absolute. The repository has many other worktrees, including older shutdown investigations. Their names do not establish current ownership or acceptance. Do not merge or clean them up as part of taking over this work.

The existing installed work was committed as `902df02` and fast-forwarded into master before the candidate branch was created. HEAD `8586abe` adopts the frozen pinned-section candidate. Published PUDL 0.104.0 adoption, candidate tests and the subsequent replay/shutdown corrections remain in the dirty worktree. The installed binary includes those changes; HEAD alone does not reproduce it.

## Latest incident and corrections

Paul twice reported that Restart left dinah-desktop.exe running and required Task Manager termination. The PUDL Team Lead chat also displayed saved history, visibly replayed a large number of updates, then appeared reordered or lost its final response. Other chat windows did not show the same dramatic replay.

The saved response was recovered. The investigation preserved the provider/saved transcript and logs before changing code. The original killed process supplied no stack, so the exact original hang location and replay timing remain unproven. There are now deterministic regressions for separate defects in the affected paths.

### Saved checkpoint and event sequencing

`cmd/dinah-desktop/frontend/chat-sequence.js` now uses a revision for checkpoint and generation resets. An obsolete in-flight journal read cannot advance the current cursor, replace its pending read or deliver into a display being replaced. Checkpoint drawing pauses reads and holds updates until catch-up. This preserves the existing sequence/generation protocol.

The first installation contained this fix alone. It corrected a demonstrated race but did not resolve Paul's subsequent report. Do not describe that first installation as a complete incident fix.

### Background history readiness

`internal/sessionhost/conversation.go` exposes `historyLoading` while the provider is replaying or restoring. A background engine open could previously report `open: true` before history and settings finished loading. Window restoration treated that as ready and accepted replay as visible live updates.

`session-lifecycle.js` now joins the existing open operation when historyLoading is true, including checks around the asynchronous saved-conversation read. `chat-applet.js` also joins when a background session event arrives at an existing window. The saved display stays visible during replay, then the completed checkpoint is adopted. The backend already deduplicates the open operation. The new flag is explicit internal metadata, not a guess based on provider text.

### Work-group isolation

Saved display copies now start with `workGroup: null` and `lastYou: null`. Adoption transfers the replacement copy's references. Previously a shallow copy retained a work-group reference from the old replay display, allowing old activity to move into the replacement checkpoint. Visual review exposed this defect, and reverting only this correction makes the added regression fail.

### Provider shutdown

`internal/acp/conn.go` now owns and closes the stdout read endpoint during Close, alongside cancellation and stdin closure. The process has `Cmd.WaitDelay` set to five seconds to bound inherited stderr-copy waits. Previously the stdout scanner could wait indefinitely when a descendant retained an output pipe, before reaching cmd.Wait.

The regression includes an open pipe whose writer stays open and a real subprocess with a descendant retaining output handles. Both fail with the original transport and pass with the correction. This demonstrates a shutdown defect without claiming that it was the exact wait in the killed native process.

## Tests and retained incident evidence

The latest evidence directory is `build/restart-replay-20261108-0521/`.

- `results.json` records the tested build and suite summary.
- `install-receipt.json` records hashes, rollback path and unchanged process IDs during installation.
- `go-before.log` and `go-after.log` retain failing-original and passing-corrected shutdown checks.
- `desktop-tests.log` retains focused desktop checks.
- `background-before.log` retains the failure to join a background open with the previous frontend.
- `workgroup-before.log` retains the failure without reference isolation.
- `browser.log` and `browser/` retain the seven-suite run.
- `browser-final.log` and `browser-final/` retain the five affected suites rerun after the final work-group correction.
- `dinah.log`, `saved-conversation.json` and `PUDL-last-response-0520.md` retain private incident evidence and the recovered reply.

All seven focused headless Edge suites passed: chat-background-replay, chat-replay-isolation, chat-saved-restore, chat-sequence, chat-snapshot-response, chat-snapshot-sequence and saved-draw-once. The final five-suite rerun passed chat-background-replay, chat-replay-isolation, chat-saved-restore, chat-snapshot-response and saved-draw-once. The final screenshot was inspected and showed the question followed by the retained response without a stale Worked group.

The internal/acp, internal/codexagent and internal/sessionhost Go packages passed. Focused desktop restart, shutdown, saved-history and replay tests passed. The production build passed. These are focused checks, not a claim of a complete repository regression or verified native Restart.

Earlier evidence is in `build/restart-response-recovery/`. It includes the initial checkpoint race reproduction, recovery copy `PUDL-last-response.md`, and before/after browser results. Its first installed build was `0.0.0-replay-fix-20261108`, SHA256 `D675DB59E15C68FB99B4D41381DB299C8FE79C8F85ABE87A8EDB520335F73D2E`, now superseded.

The full incident narrative is `docs/plans/restart-response-recovery-20261108.md`. Its historical sections retain earlier unresolved conclusions; read the follow-up and user verification sections first.

## Launcher, taskbar and terminal work already present

The title menu activates the shared PUDL launcher. First use is closed and unpinned. Launcher-pane pinning and taskbar destination pinning are separate features. Settings and guarded Quit Dinah are in the launcher header, replacing redundant visible Dinah branding. The fixed far-left taskbar Settings/Quit controls were removed at Paul's request. Settings can still appear as an ordinary pinned applet. Bare-command Quit remains unpinnable.

The launcher has a Terminals category with detected shell commands and Refresh Shells. The Tools menu was removed while preserving its commands in the launcher. Document creation/opening, workspace creation/import and Help destinations were added. Commands acting on the current document, window or agent remain in their canonical menus. Recent-work groups remain a proposed follow-up because they need workspace labels and stable navigation identities. Pin and Close are right aligned in the launcher header; Size is right aligned in the footer.

Taskbar pin eligibility uses category/provider resolution, not an individual allowlist. Applets, shells and saved items have explicit stable identity and reopening rules. Temporary pickers, diffs, result/diagnostic views and explicitly excluded kinds are not pinnable. Permissions and readiness still go through existing host entry points. Unavailable destinations retain saved membership. Preferences use the native UI-state entry `dinah-taskbar-pins-v1` and preserve the full pins/order/references envelope, separate from window-layout URL state.

Dinah combines `Title · Workspace` before passing the label to PUDL, leaving the separate detail empty. PUDL performs shortening/folding. Dinah keeps the original components separately for persistence and updates pin descriptor labels. An earlier vendor-renderer patch was removed; do not resurrect it.

A successful native terminal exit closes its window. Nonzero exit and startup failure retain diagnostics and restart affordances. Early exit is deferred through startup replay and checks that the same session remains ended. Pin membership survives the last terminal closing.

The shared PUDL Pinned section, site-menu pins, ordinary entries and Arrange toggle are integrated. The isolated host regression passed six suites, including 28 checks in two added suites. Tests cover folding at 390px, stable heights, singleton minimize/raise/restore, stacks, pictures, menu preferences, exclusions and Arrange cancellation. Evidence is in `build/pinned-section-regression/`. Initial draft-cancellation failures were fixture errors from using immediate Unpin rather than reordering; the original failures remain recorded.

See `docs/plans/dinah-taskbar-pinning.md` for identities, exclusions, test evidence and historical installation records. Its later sections supersede earlier candidate/installation statements.

## PUDL version boundary

Both `cmd/dinah-desktop/frontend/pudl/` and `internal/web/static/pudl/` contain the complete published 0.104.0 distribution with 193 files. The release tag is `1793669234fd1c4ef50bee22a41807b91ff6a11a`; the archive SHA256 is `ac9cb885e81b59528d007d9e097a95ac838e2875eacc8ebad65252e019c08a4d`. Only the pudl.css version comment differed from the previously accepted pinned-section candidate. Provenance is in `cmd/dinah-desktop/frontend/PUDL.md`; installation evidence is in `build/pudl-01040-release/`.

Later compact-navigation/responsive-launcher candidates were discussed by PUDL and Parks Computing. Those upstream and website results do not establish adoption into this Dinah worktree. Do not copy mutable upstream work or assume the latest website candidate is installed here. PUDL owns upstream release work. Verify the requested next version and its immutable artifacts if Paul asks for another update.

## Compact activity presentation

The accepted activity presentation has one collapsed Working disclosure containing nested command inspectors. Agent responses and progress commentary remain outside it. During live work the disclosure sits above the composer; completion moves the same disclosure into the transcript before the trailing response sequence. Expansion state, bounded retention, exports and reading position are preserved. Idle catch-up uses Worked and does not start a live timer. Missing historical durations are not invented.

The live panel's missing activity-list class caused the Working font mismatch and was corrected. The compact-work suite compares computed family, size and weight before and after completion. Full decisions and earlier verification are in `docs/plans/compact-command-activity.md`. This work is in the integrated baseline, not a new pending design task.

## Reproduce outside the live desktop

Use PowerShell in the install worktree. Read current local instructions and Git status before editing. The shared queue must be used for test/build commands; do not bypass a busy lock. Browser run.ps1 obtains that queue itself. Use absolute output paths, since the runner changes the working directory for suites.

```powershell
Set-Location C:/Users/paul/source/repos/dinah-install-20261105
./scripts/with-test-lock.ps1 go test ./internal/acp ./internal/codexagent ./internal/sessionhost -count=1
./scripts/with-test-lock.ps1 go test ./cmd/dinah-desktop -run 'Test.*(Restart|Shutdown|SavedConversation|SavedThrough|Replay|SessionResume)' -count=1
./cmd/dinah-desktop/frontend/e2e/run.ps1 -Only 'chat-background-replay,chat-replay-isolation,chat-saved-restore,chat-sequence,chat-snapshot-response,chat-snapshot-sequence,saved-draw-once' -Jobs 1 -Out 'C:/Users/paul/source/repos/dinah-install-20261105/build/external-replay-verification'
./scripts/with-test-lock.ps1 go build -tags 'desktop,production' -ldflags '-H windowsgui -s -w -X main.appVersion=0.0.0-background-replay-20261108' -o build/external-replay-verification/dinah-desktop.exe ./cmd/dinah-desktop
```

Use a fresh version/output directory for changed code rather than overwriting retained evidence. Quote the Go build tags in PowerShell. Inspect printed failures and failures.txt as well as process exit codes. The frontend fixtures use the real frontend with a stubbed native bridge and isolated browser contexts. They do not verify the native provider, real profile, physical touch or assistive technology.

For a native test, coordinate with Paul before controlling his desktop or restarting it. Preserve evidence before killing a stuck process if feasible. Prefer a debugger or process dump to adding temporary production logging. The original incident lacked a stack because the process was already gone. Do not mutate the real transcript to make the display match; preserve it and correct loading/order behavior.

## Installation and scope safeguards

The last install staged a verified binary, renamed the existing executable to a timestamped rollback name, moved the replacement into the canonical path, then verified its hash. The running processes continued using the prior file; process IDs were unchanged. Paul subsequently quit and relaunched, loading the replacement. Renaming succeeded on this machine; do not treat that observation as a universal file-sharing guarantee. Handle replacement failure and retain rollback.

Do not automatically commit, push, merge, publish, install or restart as part of this handoff. Prior install authorizations were executed for their specific builds. Preserve unrelated changes and stage exact paths if Paul later authorizes a commit. The untracked build directory contains private transcripts and local evidence; never stage it wholesale. Keep documentation free of em dashes and distinguish tests from user observations.

## Next actions

1. Review the dirty diff and the new tests in this worktree before changing code.
2. Coordinate one native Restart test with Paul from the installed fixed build. Capture a stack if it hangs, and verify the replacement process and transcript after startup.
3. Check that the PUDL Team Lead final response remains present and ordered after history settles, including background-open and already-open-window paths. Avoid claiming complete recovery from one superficial check.
4. If those checks pass, record the user verification and prepare a scoped commit proposal for the outstanding PUDL release adoption, tests and replay/shutdown fixes. Do not assume commit or merge approval.

The affected PUDL chat has session ID `codex:01a1a236-7111-73b3-8255-aba20eee27bd` in workspace `pudl`. No request to that agent is pending for this handoff. If bus tools are available, discover their current schema and authorization; never reuse a token copied from prior chat context.

## Working-tree inventory at handoff

The following status was captured before recording the final handoff and verification note. No existing work was committed or discarded.

```text
 M cmd/dinah-desktop/frontend/PUDL.md
 M cmd/dinah-desktop/frontend/chat-applet.js
 M cmd/dinah-desktop/frontend/chat-sequence.js
 M cmd/dinah-desktop/frontend/e2e/chat-saved-restore.js
 M cmd/dinah-desktop/frontend/pudl/VERSION
 M cmd/dinah-desktop/frontend/pudl/pudl.css
 M cmd/dinah-desktop/frontend/session-lifecycle.js
 M docs/plans/dinah-taskbar-pinning.md
 M internal/acp/conn.go
 M internal/sessionhost/conversation.go
 M internal/web/static/pudl/VERSION
 M internal/web/static/pudl/pudl.css
?? build/
?? cmd/dinah-desktop/frontend/e2e/chat-background-replay.js
?? cmd/dinah-desktop/frontend/e2e/chat-replay-isolation.js
?? cmd/dinah-desktop/frontend/e2e/chat-snapshot-response.js
?? cmd/dinah-desktop/frontend/e2e/chat-snapshot-sequence.js
?? cmd/dinah-desktop/frontend/e2e/pinned-section-candidate.js
?? cmd/dinah-desktop/frontend/e2e/pinned-section-stacks.js
?? docs/plans/external-agent-handoff-20261109.md
?? docs/plans/restart-response-recovery-20261108.md
?? internal/acp/close_pipe_test.go
?? internal/sessionhost/history_loading_test.go
```

## Subsequent update on 9 November

After this handoff was written, Paul requested the latest PUDL. Both working-tree bundles have now been updated to published 0.105.0, superseding the 0.104.0 source status above. All 193 files in each bundle match the verified archive. The six focused launcher/taskbar Edge suites pass. See the latest section of frontend/PUDL.md and docs/plans/dinah-taskbar-pinning.md for provenance and evidence. No installation, restart, merge or commit was performed for this update, so the installed build identity above remains valid.

Master was also moved to the main studio directory at Paul's request. The release-ui-pudl worktree is detached at 902df02. Conflicting local work from cleanup/history-capture was preserved in a named stash, "Preserve cleanup/history-capture work before moving master to studio 2026-11-09". Other untracked main-worktree files remain in place. The install worktree and its uncommitted fixes remain on feature/pudl-pinned-section-candidate.
