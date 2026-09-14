---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 6
note: "Verified at 7d50d5b. TestAnchorPathOfReportsNoAnchorForAKindOutsideTheGrammar in internal/bench/edit_target_test.go requires AnchorPathOf(&EntityRef{Kind: \"frobnicate\", Dir: \"/somewhere\"}) to answer \"\" and false, and requires a true second answer and a path ending in the kind's own anchor for all seven of workbench, column, card, comment, item, attachment and workstream. The wanted filenames are the format's own constants rather than AnchorOf's answers, so the test does not read the function under test for its expectation."
---
`bench.AnchorPathOf` reports no anchor for a kind outside the grammar and an anchor for every kind inside it. `TestAnchorPathOfReportsNoAnchorForAKindOutsideTheGrammar` in `internal/bench/edit_target_test.go` requires `AnchorPathOf(&EntityRef{Kind: "frobnicate", Dir: "/somewhere"})` to answer the empty string and false, and requires a true second answer with a path ending in that kind's own anchor filename for each of `KindWorkbench`, `KindColumn`, `KindCard`, `KindComment`, `KindItem`, `KindAttachment` and `KindWorkstream`.