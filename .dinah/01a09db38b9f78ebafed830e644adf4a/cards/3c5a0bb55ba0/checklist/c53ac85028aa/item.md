---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
owner: holder
ts: 2026-09-14T02:17:54Z
ordinal: 4
note: "VERIFIED. cmd/dinah/collection_reference_test.go TestAnEmptyCollectionIsAnEmptySetRatherThanAnError. Run: `go test ./cmd/dinah/ -run TestAnEmptyCollectionIsAnEmptySetRatherThanAnError -v`, on fx-2, a card whose comments directory has never been created. `show fx-2/comments` exits 0 and prints \"fx-2/comments holds nothing.\"; `contents fx-2/comments` exits 0 and prints \"fx-2/comments contains nothing.\"; `delete fx-2/comments --yes` exits 2 with dinah.is-a-collection whose --json context carries count \"0\" and no member key at all, and whose rendered sentence carries the empty splice rather than the next-member one. ARMED: made CollectionRef.Refuse's empty raise site carry the member value as well; the plant compiled and ran, and the assertion at collection_reference_test.go:286 reddened with `the refusal offers the member \"\" of a collection that holds none`; restored byte-identically, green."
---
A collection that holds nothing is an empty set rather than an error, on a card whose comments directory has never been created. `dinah show pb-2/comments` exits 0 and prints the show.collection.empty sentence naming pb-2/comments. `dinah contents pb-2/comments` exits 0 and prints the contents.empty.collection sentence. `dinah delete pb-2/comments --yes` exits 2 with refusal dinah.is-a-collection, whose --json context carries count "0" and carries no member key at all, and whose rendered sentence ends on the refusal.dinah.is-a-collection.empty splice rather than on the next-member one.