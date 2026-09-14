---
title: Should the universal archive be the extension's default install, given it is the shape that lets a stale PATH binary answer silently wrong
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
dinah-353 fixed a specific defect: the universal vsix carries no CLI binary, so it runs whatever `dinah` is on the operator's PATH, and a PATH binary built before dinah-346 answered `check` in a shape the extension misread as a bare, contentless refusal. That fix makes the misreading impossible regardless of which binary answers.

It leaves a separate, larger question untouched: whether shipping with no bundled binary is the right default at all, given that shape of install is what let a version-skewed answer reach the user silently in the first place. The platform-specific archives already carry a matched binary and cannot skew this way; the universal archive exists because the operator asked that the CLI not be bundled with the extension, and that ask stands. What is open is whether the CLI-free universal archive should still be the one a user reaches first, or whether the platform archives (or a first-run prompt, or something else) should be favored instead.

Filed as its own card on Triage's ruling against dinah-353: the detection-and-messaging fix and this distribution-default question are separable, and the second is bigger. See dinah-353 for the incident this splits from.
