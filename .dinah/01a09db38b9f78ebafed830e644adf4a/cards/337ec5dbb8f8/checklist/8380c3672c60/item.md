---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:03Z
ordinal: 8
note: "Both plants observed at c25c20f. \"six different sets of things\" to \"five\" failed with `main_test.go:7209: the table draws 6 distinct sets and the guide does not say \"they accept six different sets of things\"`. \"Fifteen commands take a reference\" to \"Fourteen\" failed with `main_test.go:7209: the table draws 15 command rows and the guide does not say \"fifteen commands take a reference\"`. Both figures stay derived from the drawn rows. assertTheGuideCountsItsOwnTable was edited in exactly the two places named: the cell count from 5 to 6, and the words slice extended from \"ten\" through \"twenty\". Restored byte-identically, test ok."
---
Change `six different sets of things` to `five different sets of things` in the guide and run `go test ./cmd/dinah/ -run TestTheReferencesGuideSaysWhichCommandTakesWhat`. It fails with `the table draws 6 distinct sets and the guide does not say "they accept six different sets of things"`. Change `Fifteen commands take a reference` to `Fourteen` and it fails with `the table draws 15 command rows and the guide does not say "fifteen commands take a reference"`.