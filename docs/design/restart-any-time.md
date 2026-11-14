# Restart without a waiting update

Pause Chats and Restart is always available in the application menu. It uses the existing pause record, turn/write admission, idle wait, two-minute notification and continuation delivery. Restart When Chats Finish remains an update command. Restart Now can finish either kind of pending restart after the existing confirmation.

The shared requestRestart function selects its target when the restart begins. If an update is ready, it selects the watched installed program. Otherwise it uses os.Executable. A later update notification cannot change that pending target. Update watching fills its metadata into the existing state so that starting the watcher cannot erase a pending same-program restart. Cancelling releases the holds and preserves the existing cancellation continuation rules.

The durable pause record adds withoutUpdate. An absent field retains the interpretation of older update records. A same-program continuation says the program restarted without claiming an update. The steering request is valid for either kind of restart. The current update state and bridge methods remain shared; no second restart protocol is introduced.

The program is restarted by its executable path with the existing --after process wait. This does not promise preservation of arbitrary launch arguments or of an old executable image if somebody replaces its file while it runs. Neither was part of the existing restart contract. Native storage persistence remains outside this change.

## Verification

The implementation is on engine/restart-any-time, based on master 5db16def. Product source is committed at 8b443882. Commit 1f398dcf corrects the menu test's row lookup; 358a43ab adds the hidden harness's optional installed-browser path. The branch has not been merged, pushed or installed.

The complete touched-package command ran under the shared FIFO and exited zero:

```powershell
pwsh -NoProfile -File scripts/go-gate.ps1 ./cmd/dinah-desktop/... -count=1
```

The desktop package passed in 173.935 seconds. Its product subpackage has no test files. The tests cover both update and same-program continuation through a fresh App, replay suppression, cancellation, new turn/write refusal, the two-minute state, the Restart Now override and a target that stays fixed when a later update appears. The launch seam verifies the same-program executable and --after arguments without launching a process.

The affected headless update.js suite passed with no page errors through scripts/with-test-lock.ps1. It checks application-menu availability without an update, the same-program status label, cancellation and the overdue card, along with the existing update journey. JavaScript syntax checks and git diff --check passed. No full browser suite ran.

The desktop was built with desktop,production tags and -ldflags '-H windowsgui' into a temporary folder. The hidden restart harness used an isolated home, DINAH_SHELL_ENTRY_FILE and headless Edge. RESTART_SAME_PROGRAM=1 leaves the executable unchanged and asserts Ready is false before pausing. RESTART_FINISH=1 releases the stand-in's turn; without it, the harness chooses Restart Now and the existing exit confirmation. Both cases exited zero, observed the old process exit and a replacement entry address, and received one continuation with the correct natural-completion or cut-short wording. Neither message claimed an update. The existing additional delivery interval found no immediate duplicate. The final scoped process check found no trial desktop processes remaining.

Evidence is in %TEMP%/dinah-restart-any-time-20261114. The finish-edge and now-edge folders contain receipts and isolated logs. The executable SHA-256 is 2e426f38ad5878ddbdc104727d5790cf66d9ffbc01fc342419ea119165f3c3bc. The natural-completion receipt SHA-256 is d3c0c1ebd22de51f47eb18dbebe4f66d42c827888de55c707c107070a17be3ea; the Restart Now receipt SHA-256 is 6c4ab946e6aa60cb0e1ffcd7771b1a1e89819262aaef2d185e07a6aeff62d845.

The first hidden attempt failed before browser launch because Playwright's matching Chromium executable was absent. Its finish folder remains in the evidence. The harness then gained an optional RESTART_BROWSER_EXECUTABLE setting and ran with installed Edge. That change affected the harness alone; the desktop executable was unchanged between attempts.

To reproduce either hidden case after building the desktop and testdata/restart-agent into a temporary folder, set NODE_PATH to a pinned Playwright installation, set RESTART_SAME_PROGRAM=1, and optionally set RESTART_BROWSER_EXECUTABLE to installed Edge. Run through FIFO:

```powershell
scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/restart-hidden.cjs <desktop.exe> <stand-in.exe> <new-output-folder>
```

The native process checks use an unsteerable stand-in with a completed first turn, followed by a working turn. Steering and cancellation use the in-process provider tests. A real model's safe-point compliance and a steerable model crossing the native process boundary remain unproved. The two-minute card is checked with controlled state rather than a real two-minute wait. The native window uses the existing hidden hook and was not visually inspected. No shared desktop input, real home, live restart or installation was used.
