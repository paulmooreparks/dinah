# Manual calibration race investigation

The calibration investigation uses engine/calibration-crash, based on c399ac9d663995c77141c4825389673c25de5a61. All executions use the shared FIFO runner and invented Ollama responses. No desktop, browser, real provider account or customer data is involved.

## Reproduction and cause

The isolated manual-calibration test passed once and then passed thirty repetitions. Running the preceding automatic-calibration and person-selected-model tests with it exposed the failure. With the documented GORACE log_path setting, the unchanged sequence reported races in judgementApp's event collector and failed in 25.229 seconds. The report names concurrent slice appends from SetJudgementSettings on the test goroutine and runCalibration's deferred completion event. It also reports a race involving growslice and the backing storage. Both event sources are legitimate.

The same unchanged sequence with default race reporting reproduced the fatal runtime failure. Its captured traceback includes runtime.winthrow, systemstack_switch on a stack outside the reported goroutine stack, and a secondary unsafe.Slice failure in runtime.hexdumpWords while printing that traceback. The test process was killed after 105.016 seconds. There is no demonstrated recursive application call chain or application stack overflow in this evidence.

The confirmed source defect is the fixture's unguarded shared event slice. The runtime traceback alone does not establish the internal cause of the Windows exception or prove that the slice race caused every runtime frame shown. The failure with default reporting and the explicit race report are retained separately. This correction removes the demonstrated race and tests the previously failing sequence without changing diagnostic routing; it does not claim a Go runtime fix.

## Correction

The fixture now serializes appends with a mutex and exposes snapshots copied under that mutex. Consumers use those snapshots, so assertions cannot race with an append or backing-array replacement. Recorded event maps are created for each event and remain immutable after insertion. The budget event assertion uses the same snapshot interface.

The new concurrent regression starts eight event writers together while another goroutine reads snapshots. It requires all 800 distinct events to be retained without duplicates. Existing event-count, state-version, calibration-model and model-source assertions remain intact. Production calibration and event behavior are unchanged. No test is skipped and no wait or timeout in an existing test is loosened.

## Verification

The original three-test sequence plus the concurrent collector regression passed ten repetitions with race detection, all vet analyzers and default race reporting in 30.309 seconds, exit zero. All forty test invocations passed. The crash-capture file was empty. The output is calibration-crash-2026-11-15/sequence-fixed-retry.jsonl.

The related calibration, judgement, budget and title tests passed together for five repetitions in 57.427 seconds, exit zero, with race detection and all vet analyzers enabled. All 95 invocations of nineteen tests passed. Their output is calibration-crash-2026-11-15/related-fixed.jsonl. The existing linker manifest warning remains. No Engine runner remains active or queued.

The first corrected build exposed a missed budget consumer and a malformed mechanical replacement in the test assertions. Both compile errors were corrected before the successful run. The failed build output remains in sequence-fixed.jsonl. An earlier diagnostic configuration used an absolute Windows path containing a colon in GORACE and was rejected before tests; repeat-before.jsonl retains that configuration error. Neither is counted as an executed test pass.

The baseline race report is calibration-crash-2026-11-15/sequence-race.76440. The default-reporting fatal traceback is default-runtime-crash.log, with sequence-default-before.jsonl recording the killed process. The baseline single-test and thirty-repeat successes remain as evidence that the preceding sequence was needed to reproduce this occurrence.

Go documents the race detector's log_path option and its report format in [Data Race Detector](https://go.dev/doc/articles/race_detector). That documented setting was used to obtain a readable race report. The final repeated passing sequence uses default reporting.

Engine retains correction ownership until receiving review accepts the candidate. Shared-inventory receiving corrections remain higher priority. No installation, push, visible window or shared-branch merge occurred.
