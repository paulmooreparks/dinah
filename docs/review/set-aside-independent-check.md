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

Additional checks cover hard-link identity, stale selections after replacement, tied fallbacks, and the manual `.before` pattern's lower fallback precedence when an `.old` file exists. Removal results are checked against filesystem identity and content. The suite will run with `go test -tags quality_set_aside ./internal/programs -run '^TestQualitySetAside' -count=1` through the shared FIFO once Engine supplies the implementation candidate. No candidate test has run, and no removal behavior is certified. This pre-implementation test commit fixes the expected results before candidate integration.
