---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:18Z
ordinal: 16
note: "`AnchorPathOf` reports a second answer so that a kind outside the grammar cannot join an empty filename and hand back the entity's directory, which is this card's own defect re-invented. What `ResolveEditTarget` then returns is a `fmt.Errorf` rather than a `contract.Refuse`, because the condition is a defect in the build rather than a mistake by the reader: no reference a reader can type reaches it, `reportError` at cmd/dinah/main.go:375 prints a non-refusal error as `unreachable` and exits 4, and `ResolvePathIn` already lets `filepath.Abs`'s own error out by that route. Minting a refusal name for it would cost a sentence in eight catalogues for a case no reader can produce, and reusing `dinah.unknown-path` would tell somebody that nothing answers to a reference that resolved. AC-6 arms the reporting arm directly rather than leaving it unexercised."
---
A kind declaring no anchor produces a plain Go error rather than a refusal.