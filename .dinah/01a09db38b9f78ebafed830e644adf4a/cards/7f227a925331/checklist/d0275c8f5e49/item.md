---
kind: open_question
state: resolved
column: aa6cd1c6ae5f
owner: holder
ts: 2026-09-14T02:18:45Z
ordinal: 37
note: "Resolved during Implement: yes, the entry names both. The shipped allowlist at internal/bench/cardreaderguard_test.go names all five under card.go at the seven-reference budget round 7 measured (LoadCard, loadRetiredCard, loadCard, LoadCardIn, loadRetiredCardIn), with the reason \"declares the three readers and the two methods that stamp what each reader answers\". Rule 2a is syntactic over result lists and both stamping methods answer a (*Card, error), so an entry stopping at the three free readers would go red on this card's own implementation. The budget is asserted in both directions on the shipped head, so the entry is neither stale nor loose."
---
The guard goes red on this card's own implementation, because `card.go`'s rule 2 entry names only the three free readers and the two stamping methods this card adds to that file answer a card. Does the entry name `LoadCardIn` and `loadRetiredCardIn` as well?