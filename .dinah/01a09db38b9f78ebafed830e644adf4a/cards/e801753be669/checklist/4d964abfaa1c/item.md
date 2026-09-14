---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:31Z
ordinal: 4
note: "Verified by internal/verb/link_test.go:TestLinkRefusesATargetTheWorkbenchDoesNotCarry, which drives both spellings (fx-99 and 000000000000), asserts the refusal name, asserts the detail is what was typed, and compares the whole anchor before and after. Armed by deleting the HasIdentifier check: red with 'link to \"000000000000\": wanted unknown-card, got ok' plus the anchor showing the entry written. Restored, green. Also seen through the binary: `dinah link ac21-1 blocks 000000000000` exits 2 with \"unknown-card this workbench carries no card 000000000000\"."
---
Given a target that resolves in neither half of the collection, `link` refuses `unknown-card` naming the value typed and writes nothing to the source card's file.