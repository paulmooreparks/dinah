---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 4
note: "Verified at 7d50d5b. editContainedShapes walks bench.Contains recursively from bench.KindWorkbench, emitting a shape for every mount and for the member the fixture put there; the collection's refusal name is read off bench.AddressedInItsOwnRight rather than restated. The coverage is also asserted rather than left to construction: mountsReachableFromTheWorkbench walks the table a second time for the expectation and requires a generated shape ending in each mount's Dir. Armed by skipping bench.CommentsDir in the generator, which printed \"the containment table declares a comment collection at comments, and the generated set carries no shape ending in that segment\"; restored and green."
---
The shape generator is derived from the containment table rather than hand-listed. `TestEveryReferenceShapeEditAcceptsNamesAFile` asserts that for every kind in `bench.Contains`'s reach from `bench.KindWorkbench`, and for every mount that kind declares, the generated set carries a shape whose reference ends in that mount's `Dir`, with the expectation computed from `bench.Contains` and never from the generated set.