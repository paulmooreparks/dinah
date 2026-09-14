---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 3
note: "Verified at 28323a1. wantEditShapes=47, wantEditOpens=24, wantEditRefusals=23 are hand-declared constants with a comment deriving them from what the fixture holds; the test asserts each against the run's own tally, asserts all three positive, and asserts the two halves sum to the total. It separately requires a shape naming each of the seven kinds, each named in its own failure message. Armed by plant 3 (deleting the two workstream shapes from Group B): the shape count reddened at 45, the opens count at 22, and the named-kind assertion printed \"no shape in the swept set names a workstream\"."
---
The sweep declares how many shapes it found and which kinds it reached. `TestEveryReferenceShapeEditAcceptsNamesAFile` asserts the constants `wantEditShapes`, `wantEditOpens` and `wantEditRefusals` against the run's own tallies, requires all three to be greater than zero, and requires the last two to sum to the first. It separately requires the swept set to carry at least one shape naming each of `bench.KindWorkbench`, `KindColumn`, `KindCard`, `KindComment`, `KindItem`, `KindAttachment` and `KindWorkstream`, each named in the failure message rather than counted.