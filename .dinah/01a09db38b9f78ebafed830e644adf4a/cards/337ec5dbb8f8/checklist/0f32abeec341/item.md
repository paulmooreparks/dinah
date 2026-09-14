---
kind: decision
state: resolved
ts: 2026-09-14T02:18:04Z
ordinal: 18
note: TestTheGuidesQuoteOnlyDeclaredRefusals at cmd/dinah/guide_guard_test.go:177 fails any dinah.-prefixed name the contract package does not declare. dinah-455 mints that name, so quoting it here would redden the suite on a card that mints nothing. The caveat paragraph says what happens in words instead, and dinah-455 may quote its own name once it exists.
---
The guide names no refusal token for a collection, so it does not write `dinah.is-a-collection`.