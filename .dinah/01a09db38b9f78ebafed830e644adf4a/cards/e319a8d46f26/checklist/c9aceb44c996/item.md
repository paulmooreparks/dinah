---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:47Z
ordinal: 10
note: "Compares the fake host's call log against the same expectations the existing tests in test/unit/cardCommands.test.ts already hold for these commands. This is the regression guard for the whole change: every test in that file predates multi-select and must stay green unedited, so a passing run of the existing suite is half the evidence and this criterion is the half that states the intent. Red when a rewrite routes the single-row case through the collecting host and swallows the toast."
---
A single-row invocation, meaning an element with the selection argument absent, behaves as it does today: on a refusal the real host's `showError` is called exactly once with `refusalMessage` of the outcome, `showInfo` and `showWarning` are called zero times, and `checkpoint` is called exactly once with that card's folder.