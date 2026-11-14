# Manual removal of set-aside programs

The implementation candidate is b49f14361e58743fe342131775839d062278d58c on engine/set-aside-programs, based on master 27827608. The contract was committed before implementation at fed971db and extended for earlier manual copies at 9579730e. The final contract is docs/design/set-aside-programs.md.

The service lists the installed programs derived from the product profile, including tool servers and forwarded commands. It recognizes both installer .old copies and earlier .before copies. An .old copy takes fallback precedence over every .before copy. Timestamp ties protect every tied newest copy. Unknown dates refuse removal rather than guess. Selection IDs bind the bin directory, name, volume/file identity, length and modification time; neither page method accepts a path.

Windows compares the identities of mapped module files with the listed candidates. The first process-image approach failed the hidden rename regression because GetProcessImageFileName reported the original pathname after a rename and thereby identified a replacement file incorrectly. The correction uses EnumProcessModulesEx and GetMappedFileName and does not assume module order. The contract cites the documented API boundaries. Process inspection has snapshot races and can be incomplete, which the response discloses. Removal uses a fresh batch snapshot, then checks each selected identity and fallback while holding a deletion handle. Ordinary FileDispositionInfo retains Windows' mapped-image refusal. No automatic cleanup was reintroduced.

## Verification

The programs package passed through FIFO in 9.570 seconds at the frozen candidate. Earlier programs runs also passed after the mapped-file correction. The original image-name implementation's failure remains recorded above; it was corrected without weakening the rename assertion.

The Windows tests start copies of the Go test executable in temporary directories, using HideWindow and CREATE_NO_WINDOW. Pipe handshakes establish readiness and closing only the fixture input pipe ends each child. One test lists an eligible copy, starts it, verifies the exact PID and identity, refuses removal, then renames the running image and puts unrelated bytes at its old pathname. The running identity follows the renamed file. Another starts a child inside the final deletion callback, after DELETE access and identity verification; ordinary disposition refuses the mapped executable and leaves it present.

Other owned tests exercise both naming patterns, precedence and ties, malformed dates, selection changes after replacement or metadata changes, malformed mutation inputs, removal of only selected eligible copies, fallback refusal, profile-derived program names, the configured isolated home, and page query/work classification. Quality owns additional independent hard-link, same-basename and service tests, frozen before reading implementation; their results belong to Quality's record.

The first isolated App test used a home environment variable while TestMain already had a temporary configOverride, so it correctly saw an empty list in that other test directory. The test now sets and restores configOverride explicitly. This was a test construction error and did not touch the real home.

The full touched suites passed through FIFO with `scripts/go-gate.ps1 ./internal/programs/... ./cmd/dinah-desktop/... -count=1`. The programs result was 9.570 seconds and desktop was 172.838 seconds; its product subpackage has no test files. The gate exited zero. Formatting and diff checks also passed.

No browser suite or product UI has been run by Engine. Front End owns the Settings presentation. No real-home files were written, no credentials or tokens were read, and no push, merge or installation occurred. Non-Windows removal is explicitly unavailable. Reported byte savings are logical lengths; hard links and compression can make physical recovery smaller.
