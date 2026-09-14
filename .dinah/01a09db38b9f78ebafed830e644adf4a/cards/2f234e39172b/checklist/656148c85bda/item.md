---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:35Z
ordinal: 6
note: "test: cmd/dinah/exithold_test.go#TestAColumnHoldingOnTheWayOutReleasesEveryOtherCard, which runs two shapes through the hold: out column, a card carrying no item at all and a card carrying an open_question naming a different column, each moved out forward and backward with exit 0 asserted on all four moves. The second shape is what catches a build that reads the hold and ignores which column the item names. Also run against the built binary: pf-2, carrying no such item, moved doing to intake and doing to review, both exit=0. observed before: fail (the helper and fixture did not exist), after: pass. Negative control for AC-4 and AC-5; the two arming proofs recorded on AC-4 and AC-5 leave this one green, which is what says it is a control rather than a restatement."
---
**End-to-end, run against the built binary:** a card standing at a column declaring `hold: out`, carrying no item naming that column at all, moves out freely in both a forward and a backward direction.