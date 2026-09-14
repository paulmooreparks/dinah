---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:11Z
ordinal: 7
note: "Verified by reading, as the criterion asks. TestEnumerateFindsBothWorkbenchesInAnAmbiguousRoot is its own function in internal/bench/rootenumeration_test.go, separate from AC-1's TestEnumerateFindsTheWorkbenchInTheRootsOwnContainer and from cmd/dinah's buildTreeFixture, and it builds its own root with two plantInContainer calls. Its doc comment names the question it answers (\"the question it answers is dinah-312 D-2: how a root whose own .dinah holds more than one recognized workbench is reported\") and states why it is not folded into AC-1's test: both mutations it guards against leave AC-1 green. A reviewer resolves this by opening the file and reading the comment above the function."
---
The new ambiguous-root fixture is its own distinct internal/bench test function (not folded into buildTreeFixture or AC-1's test), and its doc comment names which question it answers (D-2). Verified by review reading the test file, not by a run.