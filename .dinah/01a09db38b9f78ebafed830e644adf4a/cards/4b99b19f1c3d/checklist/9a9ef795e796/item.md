---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:18Z
ordinal: 9
note: "The corpus is append-only and uncategorized (162 flat entries, no existing headings above the per-entry heading). A taxonomy split needs an ongoing classification judgment at every future append and a misclassified entry becomes the same kind of defect the corpus exists to catch. Rotation needs no classification: new entries go at the tail of whichever part is open, and the only decision is a size comparison. Accepted cost: a reader hunting one specific entry still opens however many parts exist; rotation does not deliver \"load one shape's part only\" the way a taxonomy would have."
---
Repair shape is rotation by chronological position (numbered parts, each size-bounded), not a split by defect class or by catching stage.