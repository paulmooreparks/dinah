# Typed task-question host receiving

The final tested host source is cd0147462c7be3f042cc0c185c3a3793e5708352 on engine/typed-task-questions, rebased onto shipped master b265e8a7. The earlier f00dd7f1 receipts below remain historical evidence. The independent assertion file matches Quality's corrected freeze 91e82a81 byte-for-byte. Quality reviewed this source and binding in message 9808 without finding a bounded source blocker; Quality runtime acceptance remains pending the final handover.

The host accepts bounded text and offered multiple selections through reviewed AskPerson operations. The person's window submits through AnswerTaskQuestion(key, answerJSON); services receive no answering port. Existing single-option inputs, representations, result strings and assertions remain unchanged. The host rejects malformed JSON and invalid Unicode before validation, counts Unicode scalar values, preserves text and returns multiple selections in offered order. Pending typed representations retain explicit zero minima.

Pending reads now withdraw stale local task authority when a generation changed or its record disappeared. That local recheck also applies to legacy questions and is documented as a behavior change. It starts no provider or action work. No existing assertion was relaxed for this extension.

## Historical evidence

The internal task-host package passed three complete race/all-vet runs in 3.667 seconds. The initial focused desktop selection passed in 4.014 seconds with race/all-vet, including decoder and real-effect counter controls. A shell quoting error before that selection ran no Go command. The existing linker manifest warning remains.

The first tagged build request was cancelled while still queued so declaration compatibility and observation coverage could be completed before compiling. It ran no tests. The frozen f00dd7f1 tagged race binary subsequently compiled with SHA-256 65222945cc55d6e4714996ec595da2b4b2a203411c37ca401eef6a8a8f7f0ec1. All twelve unchanged real-App cases passed in 26.017 seconds. Every host exited cleanly. All twelve final artifact snapshots show zero provider/action/remote attempts and question replays, with unchanged protected records.

The broader gate retained two separate failures. Repeating the unchanged services package three times in one process panicked when its fixed registry fixture registered test-counter a second time. The task-host package passed its three repetitions in that command in 3.394 seconds, and the services package passed a fresh-process race/all-vet run. No service assertion or registry rule was changed.

The broader tagged desktop selection completed in 77.468 seconds with all ten frozen task tests passing and one supplemental pending-observation bin case refusing changed saved-chat identities. Its race diagnostics file is empty, and no data-race warning was printed. This branch predates the approved initial-state publication correction. Lead requested rebasing onto the pushed org-chart master rather than copying that correction. The attempted staging was removed before any commit. The original failed receipt remains private, and the integrated rerun remains required.

After the unchanged twelve-case control passed, all five deliberate observation mutations were detected as assertion failures. They cover an incorrect Unicode count, reordered selected IDs, a consumed invalid answer, duplicate settlement and an erased pending obligation. The mutation driver itself exited zero with detected=5 and expected=5. The original host responses remain in separate private artifacts. The real allocation/action/refused-and-successful document-write counter controls also passed in the broader tagged run. These results use Go 1.26.10 on Windows amd64.

The binding retains actual Review, person-confirmed Begin, permit inputs, operation identity, receipt persistence and trusted App submission. Exact malformed bytes enter the Go process without normalization. It mutates copied pending values inside that process and synchronizes concurrent submissions at the actual authority check. Restart reconstructs the App and tries Rebind and the same operation again. All provider calls, notifications, prompts and preparation attempts are observed; action and remote-document attempts use the previously accepted counted peers. Protected local/remote documents, drafts and links remain in the snapshot. Separate controls must prove the counters before the five deliberately altered captured observations are tested against the unchanged assertions.

The current-base producer gates below replace the earlier pending integrated run. Raw paths, toolchain output and fixtures stay in private temporary artifacts. The public branch keeps only this summary and executable bindings. Rendered Notifications, Decisions, Flow and installed-product acceptance remain outside this host result.

The separate Flow admission lookup contract is recorded at fork commit 9bb123eb on engine/flow-inventory. It permits passive BeginOutcome lookup after a lost response and preserves an unresolved admission without another Begin.

## Current-base handover

The clean rebase retains every reviewed implementation and binding blob and the unchanged frozen assertions. Quality confirmed that comparison in message 9830. The base supplies the shipped session publication correction, with no duplicate correction commit.

All seven pipeline commands completed successfully under one canonical shared pwsh lease using Go 1.26.10 on Windows amd64. The tagged race/all-vet host compiled in 22.245 seconds. Its SHA-256 is faddbe0cfaf7848709be4da1dc66efe48f2539ffca07fdb50e1871dc62acde86. The twelve unchanged real-App cases passed in 25.481 seconds of wall time, with clean child exits. The internal task-host package passed three race/all-vet repetitions, and services passed once in a fresh process.

The integrated tagged desktop race selection passed in 50.432 seconds of wall time. It includes 22 top-level cases and 51 passing cases when subtests are counted, including all ten frozen task-host cases, legacy question gates, the previously failing supplemental bin observation and the real counter positive controls. After the unchanged controls passed, all five deliberate observation mutations were detected. No data-race warning or panic appeared. All 21 runtime crash capture files were empty.

The complete desktop non-race/all-vet package passed in 210.131 seconds of Go package time (219.733 seconds of wall time), with 987 passing tests and subtests. The original old-base failures remain recorded above. No assertion, retry policy or timeout was loosened to obtain these results.

The public branch retains executable bindings and this summary only. Raw compiler output, JSON receipts, isolated homes and mutation artifacts remain private. Quality is asked to confirm bounded runtime clearance from these receipts and its prior source comparison; Lead is asked to integrate the current-base branch after that clearance. This handover covers the host extension only. Rendered Notifications, Decisions, Flow and installed-product acceptance remain outstanding.