---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:52Z
ordinal: 20
note: "The field is empty for such an item today, and its doc comment gives the reason that nothing would resolve a reference composed from one. Verified false at 22a35fc: a hand-written item.md carrying kind risk at position 3 is answered by `dinah show pb-1/checklist/3`, because walkBelowCard narrows by kind only for the three aliases and otherwise descends unnarrowed. The row that results has a blank reference cell under `dinah show`, which is the defect this card exists to remove standing on the very block dinah-435 fixed. The doc comment is replaced along with the behaviour."
---
verb.ItemView.Ref becomes total: an item whose kind is none of the three declared kinds is addressed as `<card>/checklist/<n>`.