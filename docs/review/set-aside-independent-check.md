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

The Engine branch and design note were not present when this record was created. Quality has requested the contract path and revision. No candidate test has run, and no removal behavior is certified.
