---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:03Z
ordinal: 10
note: Planted and observed at c25c20f. Copying the references guide's "A reference or a query" paragraph verbatim into query.md in place of the reciprocal failed six times, one per shared sentence, at guide_guard_test.go:1013, the first being `the guides references and query both carry the sentence "you name a thing with a reference and you find things with a query", so one of them is answering the other's question`. Restored byte-identically and the test is ok with the two paragraphs as the spec writes them.
---
Copy the guide's `A reference or a query` paragraph verbatim into `internal/guide/guides/query.md` in place of the reciprocal, and run `go test ./cmd/dinah/ -run TestNoSentenceStandsInTwoGuides`. It fails naming the shared sentence and the two guides. With the two paragraphs as the spec writes them it passes, which was checked before the spec was written by running that test's own guideProse rule over the whole shipped corpus with both texts in place.