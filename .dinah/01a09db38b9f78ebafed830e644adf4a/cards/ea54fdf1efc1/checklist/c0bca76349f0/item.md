---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 16
note: "dinah-456 section 5.3 requires that a state write admit only the values the item's kind admits and record the event the matching verb records. Routing to `Library.Resolve`, `Verify`, `Fail` and `Reopen` satisfies both without a second implementation, and it brings the two checks a reimplementation would have missed: `dinah.wrong-item-kind` and `dinah.not-pending`, plus the citation obligation an acceptance criterion carries before it may leave pending. Those verbs take a note or a reason, and `set` has nowhere to put one, which is why `--note` is minted; it is refused beside any other field under `dinah.usage`, the shape `--at` already has on a card's tier. The two spellings are one act: `dinah set <item> state verified --note x` and `dinah verify <item> x` write the same journal line."
---
A write to an item's `state` routes to the checklist verb that lands that state, and `set` gains a `--note` flag legal only beside that field so the verb's note reaches it.