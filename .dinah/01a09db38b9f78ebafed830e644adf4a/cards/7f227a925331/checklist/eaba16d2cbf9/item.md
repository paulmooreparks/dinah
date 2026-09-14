---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:44Z
ordinal: 26
note: A card's file no longer carries its number, so a function taking a root and an identifier cannot honestly produce one. The migration, the checker, and the tests all need to read a card without a registry, so the free function stays exported and keeps that job. Twenty-five non-test call sites move to the bench method at c1ae4a9, seven in internal/bench and eighteen in internal/verb, plus one reference to the function as a value at bench.go:2074. The guard enumerates resolved identifier nodes rather than matching text, because a text search for `bench.LoadCard(` sees only the eighteen and a bare-name search would also match LoadCardIn and loadRetiredCard; round 3 added the dot-import case to the `*ast.Ident` rule. bench.go needs no allowlist entry, because cardsIn is deleted rather than rewritten (a free function taking a root reaches no registry), which removes the function value at :2074, and NextNumber's rewrite removes the call at :2130; Bench.Cards becomes cardsWith(b.CardsRoot(), b.LoadCardIn). LoadCardIn also needs an exemption entry in internal/addressform/addressform.go beside the LoadCard entry at :370, or TestEveryAddressAnsweringFunctionIsRosteredOrExempted (cmd/dinah/address_form_arms_test.go:221) goes red. AC-12 drives the scan and AC-13 drives the observable that the scan cannot reach.
---
`LoadCard` stays number-free and a new `Bench.LoadCardIn` stamps the number, with an AST-based enumeration guard over every reference to the free function.