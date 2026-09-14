---
kind: decision
state: resolved
ts: 2026-09-14T02:16:53Z
ordinal: 16
note: "Section 4 defines Layer as a body of declarations outside the profile, named so it cannot collide with it, which is section 9's mechanism: a declaration in the workbench definition under a dotted name. Dinah's user-global instruction file is a file on disk and not such a declaration, so a bare \"layer\" in this paragraph tells a reader that Dinah declares layers. That is exactly OQ-2's confusion, planted by the paragraph whose job is to stop a reader concluding something false. CORE-INSTR-6 already writes \"one instruction layer into another\", so the compound is the document's own word and needs no new vocabulary. Raised by Agent Design Review, comment 5869."
---
The section 7 paragraph says "instruction layer" everywhere the noun appears, and never bare "layer".