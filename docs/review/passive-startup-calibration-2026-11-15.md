# Passive judgement-state correction

Front End's first combined case found one Ollama tags request and one process-list request after the page deferrals. Opening the judge store still scheduled automatic calibration. Lead confirmed in message 9157 that store and state reads must remain passive even when judging is enabled.

The correction removes calibration from `judges()`. The four real judgement paths request due calibration through `judgesForCall`, with disabled kinds excluded. Their existing rule result remains available while calibration is pending or no model is selected. The visible settings inventory also requests due calibration. Explicit manual calibration retains its existing entry point and checks. Calibration cannot begin after judge shutdown starts.

## Verification

The new counted Ollama fixture enables automatic calibration without the test-only `noAuto` suppression. State, saved settings, report and store reads produce zero HTTP requests. A real chat-title judgement then triggers inventory, and its rule result remains recorded. A second fixture proves that an explicit settings inventory can trigger calibration.

All 26 tests from the affected judgement, calibration, title, document-name, handoff and set-aside files passed without race instrumentation, with all vet analyzers enabled, in 5.369 seconds. The two new trigger tests and the automatic-calibration test passed with race instrumentation and all vet analyzers in 2.342 seconds. Full desktop vet passed separately. Source was unchanged during these final runs. The existing linker manifest warning remains.

The first new fixture assertion incorrectly equated the widget's `on` state with the saved Enabled flag. The widget says `off` when no model is chosen, so the corrected fixture checks Enabled directly. The zero-request assertion did not change.

A broader race selection exceeded its budget. The subsequent 26-test JSON run captured a fatal Windows Go runtime stack error during the existing manual-calibration test, followed by a timeout. The capture reports an invalid return PC and `unsafe.Slice: len out of range` while printing runtime stacks. Its origin remains undiagnosed. Neither run establishes a complete race pass. The tests and assertions were retained; no timeout was extended to obtain a pass.

The interrupted race trace, runtime crash capture, complete non-race trace and focused race trace are retained outside the public repository because they contain machine-specific paths. These fixtures use invented local state and providers. No real account or native program was contacted during that host verification.

## Receiving action

Front End should take this correction over host `35a56f44` and rerun the unchanged enabled-but-hidden zero-contact case, followed by the combined selection. The host API and DTO are unchanged. Quality's independent combined gate remains required. Native Claude adapter maintenance remains outside the page-startup claim and is still assigned as a separate stage.
