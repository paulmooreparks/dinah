---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:31Z
ordinal: 6
note: "Verified by internal/verb/link_test.go:TestLinkTakesAnyKindAndCanonicalisesNothing, which drives tile-order-duplicates, relates, Blocks-Drywall and \"came out of\" and requires each stored kind to equal what was passed. Armed twice: a closed set of four permitted kinds went red at the first invented word (\"refused malformed\"), and a canonicaliser alone went red naming the rewrite ('stored the kind \"blocks-drywall\" rather than \"Blocks-Drywall\"'). Restored from the commit, green."
---
`link` accepts any non-empty kind string; a kind absent from format.md's suggested spellings (`duplicates`, `relates`) succeeds identically to one that uses a suggested spelling, and no vocabulary check ever refuses on the kind's spelling.