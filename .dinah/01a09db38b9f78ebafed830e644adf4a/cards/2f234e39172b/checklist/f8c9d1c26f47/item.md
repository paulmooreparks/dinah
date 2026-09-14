---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 26
note: "No, not without a second guard: a non-operator could clear or overwrite an item's owner field before closing it, defeating the new refusal entirely, which is the \"refusal any value satisfies\" shape this board has been burned by before. Fix folded into this card: writing an item's `owner` field is refused to a non-operator whenever the item's stored owner currently is (or is being set to) \"operator\". Filing a NEW item with `--owner operator` stays open to any owner, since that is the ordinary routing-to-the-operator act; only changing an existing operator-owned item's owner is restricted."
---
Found while specifying rather than asked for: does closing-refusal alone actually protect an operator-owned item, given that an item's own `owner` field is otherwise writable by any owner?