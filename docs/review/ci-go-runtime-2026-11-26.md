# Full Go suite runtime and fixture waits

This investigation uses build run [45756940230](https://github.com/paulmooreparks/dinah/actions/runs/45756940230), source `01d52ca5`, and its full Go job `139697778959`. The failed job log remains in `%TEMP%/dinah-ci-45756940230-job.log`. The downloaded diagnostic artifact remains in `%TEMP%/dinah-ci-45756940230-diagnostics`.

## Package budget

The desktop test inventory contains 926 top-level tests. The runner completed 382 before the ten-minute package limit stopped it. Their recorded durations sum to 599.77 seconds. The last test, `TestClaudePicturesAreReducedTo2000Pixels`, had started only 80 milliseconds before the package failed. No completed desktop test failed. These observations show cumulative suite runtime rather than a ten-minute stall in that final test.

The slowest completed top-level tests were `TestChatQuestionSteersActiveTurnOrFallsBackOnce` (31.90 seconds), `TestAskOpensAClosedChatInTheBackground` (17.89), `TestCleanupResumesEveryDurablePhaseAfterRestart` (16.65), `TestAskReconnectsARestoredWindow` (12.68) and `TestBusyBusDeliveryUsesSteeringCapability` (12.67).

A complete local JSON timing run passed in 360.762 seconds, with 918 top-level passes and eight existing capability skips. The same 382 tests completed on the runner consumed 193.73 seconds locally, versus 599.77 seconds on the runner. The local slowest top-level test was `TestChatQuestionSteersActiveTurnOrFallsBackOnce` at 10.23 seconds, followed by `TestEveryProviderTurnReceivesCommunicationProtocol` (7.88), `TestQualityPauseProviderContract` (7.71), `TestNativeListedChatCanBeBinnedRestoredAndDeleted` (6.16), and `TestRemovingAChatTakesTheEarlierStagesItsSavedConversationsLink` (5.13). The full local timing output remains in `%TEMP%/dinah-ci-runtime-desktop-before.jsonl`.

The workflow now passes `-timeout=30m` to the full Go suite. At the measured completion rate, 926 tests would take about 24 minutes; thirty minutes gives that estimate some margin while retaining a finite package limit. This is a budget estimate, not a claim that all tests have equal cost. The job still runs every package once, retains per-test JSON and restricted diagnostics, and propagates the test exit status. Its guard expects the new command without introducing a skip or conditional exception.

## Node fixture selection

The Go job previously inherited Node v22.23.5 from the Windows runner, while the browser jobs explicitly selected Node 24. Both terminal failures occurred before the fixture recorded its `entered-js` phase, including the ordinary-handle journal test that does not start PowerShell. This evidence does not establish a PowerShell or Ctrl+C delivery defect. The ordinary-handle `--version` preflight passed in 608 milliseconds. The native artifact contains initialization frames, but also states that Node PDB resolution is not established; it is diagnostic evidence rather than a proven upstream cause.

Both unchanged terminal tests pass locally with Node v24.4.1 in 2.366 seconds. An isolated download of the runner's Node v22.23.5, checked against the release's SHA-256 manifest, also passes in 3.185 seconds. Five further runs with Node v22.23.5 during the local desktop timing run also passed, in 8.741 seconds, recorded in `%TEMP%/dinah-ci-runtime-node22-repeat.log`. That download lives only under `%TEMP%/dinah-node-22.23.5-fixture`; no runtime was installed and no user environment was changed. The respective logs remain in `%TEMP%/dinah-ci-runtime-terminals-before.log` and `%TEMP%/dinah-ci-runtime-node22.log`.

The Go job now explicitly selects Node 24 through the same pinned setup action as the browser jobs. No terminal assertion, startup deadline, Ctrl+C check or capability skip changes. This removes the uncontrolled runner-image runtime difference. The runner-only initialization failure remains unconfirmed until the changed job runs; a local pass of either version cannot establish its cause or its resolution.

## Local load failures

Three reported local failures were fixture completion waits. The document service had not announced itself before its five-second polling limit; the task receipt had not settled in three seconds; a chat-open outcome had not arrived in five seconds. Each case passed alone immediately afterwards. The original complete local logs were overwritten, so only the reported failure lines are available.

The service-announcement helper, receipt waiter and chat-open outcome waiter now use the test lifetime context instead of independent speed thresholds. They still require the actual announcement, settled receipt or emitted outcome. Explicit behavior and latency assertions elsewhere remain unchanged. The enclosing Go package timeout bounds the tests. This uses the documented [testing.T.Context](https://pkg.go.dev/testing#T.Context) contract and changes no production deadline.

All three reported cases pass five times with Go 1.26.11. The package results are 1.287 seconds for docservice, 2.211 seconds for taskhost and 11.220 seconds for the desktop selection. Their output remains in `%TEMP%/dinah-ci-runtime-load-fixtures.log`. Workflow and Node-preflight tests also pass; their output remains in `%TEMP%/dinah-ci-runtime-workflow.log`.

Only CI configuration, tests and this record change. No UI or product implementation changes. Remote build and publish clearance still require the integrated workflow to pass.

The final full `scripts/go-gate.ps1 ./... -count=1 -timeout=30m -json` run passed with Go 1.26.11 and exit zero. Desktop completed in 438.818 seconds, with 918 top-level passes and the same eight existing capability skips. Docservice passed in 7.961 seconds, taskhost in 3.247 seconds and terminals in 17.364 seconds. No terminal assertion was skipped in this local run. The complete JSON output remains in `%TEMP%/dinah-ci-runtime-final-go.jsonl`. The ordinary local desktop timing run and this full run both finished without a hang.

Remote receiving in run 45764486029 subsequently passed the terminal package in 25.1 seconds with Node 24. The desktop package completed in 673.7 seconds, inside the new budget. The remaining completed-test failures are investigated in `ci-contract-fixtures-2026-11-26.md`; the earlier Node and package-budget blockers did not recur in this run.
