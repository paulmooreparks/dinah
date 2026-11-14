# Independent set-aside removal check

Quality owns this check on `quality/set-aside-check`, based on master `27827608`. The requirements below were recorded before reading implementation code. The service contract is still awaited; its API shapes will determine the executable test adapter. No implementation code has been inspected.

The test uses an isolated temporary home and invented program files. A helper executable signals readiness through a temporary file and waits for a stop signal. It runs hidden and never takes desktop input. Readiness is observed before listing or removal, with a deadline used only to report failure. Every child is stopped and awaited during cleanup.

## Required cases

1. A process starts from an older set-aside copy while a newer fallback exists. Listing must identify the older file as in use. Removing either protected file must be refused, and their file identities and contents must remain unchanged.
2. Two directories contain programs with the same basename. Only one file is running. The running file must be protected by identity, while an eligible older file in the other directory remains removable. Each program retains its own newest fallback.
3. An eligible copy is listed, then a process starts from that copy and signals readiness before removal. Removal must recheck its state and refuse it. The original list cannot authorize deleting the now-running file.
4. An eligible older copy is explicitly selected and removed. Its unselected siblings and every fallback remain unchanged. This prevents a service that refuses every request from passing the protection checks.
5. The newest fallback is selected directly even though the UI disables its selection. The service must refuse it. Protection must hold without trusting the page.

Each refusal is checked against both the service result and the filesystem. The positive removal checks the selected file's absence and the untouched files' identities and contents. The test records counts and assertion results, without reading or changing any installed program or real home.

The contract must specify how program directories are supplied, how a listed file is selected for removal, how refusal is represented, and how the newest fallback is ordered. Tests will use those documented shapes without deriving expected behavior from the implementation. The completed test source will be committed before candidate implementation code is read or integrated.

## Current status

The executable tests are written against the amended contract at `9579730e`, read through its attachment before implementation inspection. They reside in `internal/programs/quality_set_aside_windows_test.go` under the opt-in `windows && quality_set_aside` build constraint. They use the documented exported service directly, with no mock of process discovery or deletion. The child is a copy of the test executable with hidden startup, a stdout readiness handshake and a stdin lifetime pipe.

Additional checks cover hard-link identity, stale selections after replacement, tied fallbacks, and the manual `.before` pattern's lower fallback precedence when an `.old` file exists. Removal results are checked against filesystem identity and content. Commit `e60c4b7f0b0f0228b794d4fd2608e1a5024bd276` froze these expectations before candidate integration.

## Independent candidate result

Engine candidate `b49f14361e58743fe342131775839d062278d58c` and its two contract commits were cherry-picked into the isolated Quality branch. Its implementation files matched the candidate, and the independent test file matched the frozen test commit. The first run passed unchanged through FIFO ticket 1571 with `go test -tags quality_set_aside ./internal/programs -run '^TestQualitySetAside' -count=1 -v`. The command exited zero in 20.880 seconds of package time, excluding the queue wait.

| Independent case | Result |
| --- | --- |
| Running set-aside image, same basename in another directory, eligible removal and fallback preservation | Passed in 5.80 seconds |
| A copy starts running after its removable selection was listed | Passed in 2.27 seconds |
| Manual naming pattern, old-copy precedence, tied fallbacks and manual-only fallback | Passed in 7.90 seconds |
| A replacement invalidates the previous identity-bound selection | Passed in 1.99 seconds |
| A process runs through a hard link with a different filename | Passed in 2.43 seconds |

The helper entry point also passed. Hidden children acknowledged readiness before each operation and were stopped and awaited during cleanup. All files belonged to temporary test directories. No test assertion, timeout or expected refusal was changed to obtain this result.

This independent run covers the exported service on Windows. It does not cover Front End's Settings confirmation, layout, or bridge integration. The after-listing case does not independently force a process start inside the final deletion call; Engine's separate internal-hook test addresses that narrower interval. No full repository suite, non-Windows runtime test, installation or real-home operation was performed. The revised mapped-module contract was read after the tests were frozen; its stated safety behavior remains consistent with these assertions.
