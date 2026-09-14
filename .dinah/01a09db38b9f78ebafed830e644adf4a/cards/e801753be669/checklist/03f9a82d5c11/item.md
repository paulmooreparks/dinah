---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:31Z
ordinal: 2
note: "Verified by internal/verb/link_test.go:TestLinkWritesOneEntryToTheCardsOwnAnchor, second assertion: it fails if the anchor contains \"to: <the reference typed>\" rather than the resolved identifier. Armed by making ResolveLinkTarget return raw on the ResolveCard branch; red with 'stored the reference \"fx-2\" under to: rather than the identifier \"0444e8b1a886\"'. Restored, green."
---
Given a target typed as a human reference that resolves in the live half of the collection, `link` stores the target's resolved 12-hex identifier under `to:`, not the reference text typed, verified by reading back the raw `to:` value.