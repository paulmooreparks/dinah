# Contract-driven CI fixture corrections

Build run [45764486029](https://github.com/paulmooreparks/dinah/actions/runs/45764486029), source `2ca62dc4`, completed the desktop package in 673.7 seconds. Node 24 cleared both terminal failures. The remaining failures investigated here are retained in `%TEMP%/dinah-ci-45764486029-failed.log`; the full Go job is `139721992094`.

## Restore completion

The two restore tests used a 40-millisecond restore timer but rejected an end-to-end open that took more than three seconds. That elapsed time included host setup and persistence. The runner's 4.379-second reopen and 13.630-second new-chat operation did not establish that either operation waited for the held provider reply.

The provider now signals when its restore request has entered the held callback. After the open returns, each test requires that signal and proves that the provider's reply is still blocked. This directly verifies opening without that reply. The 40-millisecond override remains, as do the production restore limit and all ready-state, retained-choice, single-request, notice and late-answer assertions. No machine-speed threshold replaces the old one.

## Provider command prerequisite

Both the Devin and plain pause modes use the Devin provider. Their real adapters still validate the managed hook command even though their local wire peer does not execute it. Each now has its own disposable command beside the simulated window and an empty PATH, after locating Node by its absolute path. The executable-location hook is restored at cleanup. The pause, persistence, fresh-App recovery and carry-on assertions remain intact for all five provider modes.

## Cancellation result publication

`CancelTurn` waits for the registered turn's internal completion signal. The test then used a nonblocking read to require the caller goroutine to have forwarded its returned value through a different channel. The internal signal does not order that subsequent send, so a correct cancellation could fail this scheduling assumption.

The test now holds that forwarding behind an explicit gate. Cancellation must return while forwarding is held, and a second cancellation must report that the port has no running turn. The test then releases the caller and waits for its result, requiring the cancelled stop reason, nil error and original exact prompt/cancel counts. Production cancellation code is unchanged.

## Workspace identity

Workspace inspection had a production comparison defect. `taskWorkspace` resolves its execution path, while the inventory comparison used the chat's original path spelling. A directory alias therefore produced an empty inventory for the correct workspace. A local directory-symlink regression reproduced the same empty result before the correction; its failed output remains in `%TEMP%/dinah-ci-inspection-alias-before.log`. The runner's short-name temporary path is consistent with this mismatch, although its failure line did not print the two paths.

Inspection now resolves absolute chat workspace paths before comparing them with the resolved task binding. Unresolvable, blank and relative paths are excluded. The regression retains a foreign workspace and verifies that it is excluded. Further controls change the process directory to the task workspace and prove that blank and relative chat paths still cannot enter its inventory. The original checks still require no provider calls and no loaded runtime sessions. The correction changes no UI code.

## Verification

With Go 1.26.11, all five selected top-level tests passed three times, including all five pause modes, in 84.930 seconds. Their output remains in `%TEMP%/dinah-ci-contract-focused.log`. The restore, cancellation and inspection tests also passed three times under the race detector in 18.872 seconds, recorded in `%TEMP%/dinah-ci-contract-race.log`. The final inspection controls passed three times under the race detector in 6.378 seconds, with both alias and unscoped-path cases executed, recorded in `%TEMP%/dinah-ci-inspection-final-race.log`.

Only one production file changes, for the workspace identity comparison. All other code changes are test fixtures. The graphical interface is unchanged, so no browser checks apply. Remote build and publish clearance still require the integrated workflow to pass.

The full `scripts/go-gate.ps1 ./... -count=1 -timeout=30m -json` run passed all 73 packages with exit zero and no failed tests. Desktop completed in 525.032 seconds. Its output remains in `%TEMP%/dinah-ci-contract-full-go.jsonl`. That run exercised the production correction and the original contract regressions. Its desktop binary was built before the final unscoped-path subcase was added; that final test-only addition is covered by the separate three-pass race run above. The integrated receiving gate must run the complete final tree.
