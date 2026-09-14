---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:09Z
ordinal: 12
note: dinah-456 section 5.1 rules that every field a person authored on any kind is readable and writable, and `dinah file` accepts both `--column` and `--owner` at internal/verb/definition.go:360, storing both on the item. The parent's section 5.3 table carries `owner` and omits `column`, and nothing in that spec, its decisions or its comments says why the two flags of one command part company. Read as an oversight rather than a ruling. If the reviewer reads it the other way, this is one name in one table and one row of the sample table beside it. Spec section 3.4 states the addition where the implementer will meet it.
---
An item's gate column is settable, so the item field set is `text`, `state`, `note`, `owner`, `column` rather than the parent's four.