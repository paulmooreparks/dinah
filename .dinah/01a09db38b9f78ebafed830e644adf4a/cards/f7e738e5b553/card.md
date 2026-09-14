---
title: Nothing finds a test that spells the profile version out
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Amending the `dinah-core` profile moves its version, and the only way anyone has found the tests that assert that version is by watching them go red.

Two rounds of evidence:

- dinah-253 moved the profile 0.5 to 0.6. Its spec named the tests it expected to touch and missed three, all asserting the same version window: one in `internal/bench` and two in `cmd/dinah`. The implementer found them by running the suite.
- The same bump broke two VS Code extension integration tests, `editors/vscode/test/integration/suite/active.test.ts` and `.../carried.test.ts`, which assert the profile string the live binary reports. Those jobs are not required checks, so the merge landed with them red and they stayed red on the trunk until dinah-284 fixed them.

Five instances across two languages, found twice by accident. The question this card exists to settle: should a test ever be allowed to name a profile version literally, and if so, what finds them all on the next amendment.

There is a real distinction to preserve. A fixture literal feeding a parser is legitimate and should stay: the unit tests under `editors/vscode/test/unit/` spell out a version because they are testing the parser, not the tool, and rewriting them to track the live version would weaken them. What is wrong is an assertion ABOUT the running tool that restates a number some other card is entitled to change.

Candidate shapes, none chosen: a guard that greps for the version pattern and requires each occurrence to be annotated as a deliberate fixture; a single declared constant that every live-version assertion reads; or a documented convention plus the amendment checklist naming where to look. The first two are enforceable and the third is not, which probably decides it.

Whatever lands should also cover the amendment procedure itself, since the recurring failure is that a spec author cannot see the full set at spec time.

Split out of dinah-284 at triage, which correctly kept the narrow unblocking fix travelling alone.
