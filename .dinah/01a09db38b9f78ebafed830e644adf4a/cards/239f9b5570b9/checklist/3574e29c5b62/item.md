---
kind: decision
state: resolved
column: 4b38abe7ebd5
ts: 2026-09-14T02:17:54Z
ordinal: 40
note: Renamed to bench.WordForItemKind. After this card the noun "alias" means the short form, which is the one spelling this function must never return, so leaving the old name would have left the codebase's clearest statement of the rule saying the opposite of the rule. The rename is mechanical and reaches two call sites, internal/verb/read.go's itemRef and the round-trip guard in internal/bench/items_test.go, and the same reasoning carried the word "alias" out of six comments in read.go, tree.go, render.go and the two guides. Nothing outside internal/bench and internal/verb named it, and the exported surface is not published anywhere with a compatibility promise.
---
Rename bench.AliasForItemKind, or leave the name and change only what it returns?