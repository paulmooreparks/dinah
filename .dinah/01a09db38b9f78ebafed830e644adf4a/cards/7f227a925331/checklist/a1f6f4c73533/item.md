---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:45Z
ordinal: 31
note: "Round 3's spec said `retiredCardsIn` was untouched and that its cards' numbers came from frontmatter through the legacy branch, and the tree says otherwise: `Bench.Cards` routes a lenient-opened workbench through `retiredCardsIn` (`bench.go:2079`) to `loadRetiredCard`, which never passes through `LoadCardIn`, so a fallback living there would have left every card of every retired-vocabulary workbench carrying Number zero and every reference of theirs rendering as its hex identifier. `Bench.stamp` now holds the one branch, reading the registry at RegistryFormat or above and the card's own frontmatter key below it, and both `LoadCardIn` and a new `loadRetiredCardIn` call it. `cardsIn` and `retiredCardsIn` are both deleted, `Bench.Cards` passes the two method values to the unchanged `cardsWith`, and the addressform roster takes four edits rather than two, since `TestEveryAddressAnsweringFunctionIsRosteredOrExempted` errors on an exemption that excuses nothing as well as on a function nothing names."
---
The retired-vocabulary read path gets its own stamping method, and the frontmatter fallback lives in one shared helper rather than inside `LoadCardIn`.