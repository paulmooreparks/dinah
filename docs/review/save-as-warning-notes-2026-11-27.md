# Save As completion warnings

Save As completed a document copy but discarded the backend's omission notes. The agent save-to-path completion discarded the same notes before remounting its window. Both completions now display each returned note through the existing warning component with keep enabled, matching Publish. Backend copy policy, returned text and agent output shapes are unchanged.

The regression runs the real packaged Editor, Save As picker callback, content host and agent page routing against an invented draft service. Its Markdown references missing-invented.png, and the service returns two omission notes after completing the copy. Both person and agent paths must show each note once, retain it through the existing keep option, render its warning toast and still return the saved document. Empty and absent notes must show no warnings. The cases run in light and dark themes.

On unchanged dd6032fe, the tests fail twelve warning observations in seven seconds while completed copies and empty/absent-note cases pass. The same tests pass after the two frontend warning calls are added. The five affected suites pass in 25 seconds: content-save-notes, content-save-outcomes, content-action-outputs, content-host-location and documents-menu. There are no page errors. The hidden, headless runs used the shared FIFO test queue.

Private logs and screenshots are retained in `%TEMP%/dinah-save-notes-red` and `%TEMP%/dinah-save-notes-green`. The tests use an invented draft service and do not establish native picker or installed-provider acceptance. No backend refusal policy was changed, and incomplete copies still follow the backend's existing behavior.

The branch is frontend/save-as-warning-notes, based on dd6032fe. The Integrator should receive it under docs/guides/merge-and-install.md and rerun the five affected suites. The handover supplies the staged guard and branch scan results. Front End retains correction ownership until acceptance. No merge, push or installation was performed by Front End.
