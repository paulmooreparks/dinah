---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:36Z
ordinal: 19
note: "`hold` is not replaced or split. Its typed name, stored key (`gate_items`) and guard (`GuardHold`) are unchanged, and its vocabulary grows from two values to four: `on`/`off`, unchanged in every respect from dinah-477, plus two new values, `out` (holds on the way out) and `both` (holds both ways). `on` continues to write `gate_items: true` and continues to mean \"holds on entry,\" so an existing declaration sees zero behavior or meaning change. This supersedes the prior revision's sibling-field decision (`ExitHold`/`gate_items_exit`), which Agent Design Review rejected as narrower than the operator's own words: a direction is one property with an axis, not two independent switches."
---
Is `hold` (dinah-477) replaced, reinterpreted, or left untouched, and how is the new capability added?