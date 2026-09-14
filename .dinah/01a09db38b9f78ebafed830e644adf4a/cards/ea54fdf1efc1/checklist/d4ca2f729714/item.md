---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 14
note: "The reuse was considered and it is cheaper: no new constant, no format-document rows, no widening of `contract.Events`. It was rejected because `card_updated` would then answer two questions at once, \"the card's own field changed\" and \"something below the card changed\", and `contract.Events` is the vocabulary `dinah query event:<name>` accepts, so the merge would delete a query a person can write today. The `*_updated` family is already per kind and `column_updated` already sets the precedent for a kind-named event landing on somebody else's journal and naming its subject in `note`. Spec sections 5.1 to 5.3 carry the shape and the cost, including the fixture recapture, which is owed either way because the member set of a line changes on both designs."
---
Three journal events are minted, `comment_updated`, `item_updated` and `attachment_updated`, rather than reusing `card_updated` for everything that lands on a card's journal.