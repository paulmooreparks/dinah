---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:21Z
ordinal: 26
note: "Three hand-counted figures in this card came back wrong on review: round 1's \"pins three\" prose-ledger entries, which is four; round 2's report claiming six rewritten items while naming nine; and round 2's \"seven of the ten\" summaries carrying the clause separator, which is five in every one of the eight catalogues. Each supported an argument that was itself sound, which is why they survived. The remedy is the one the workbench instructions already carry with `scripts/derive_event_counts.py` as its worked example: compute the figure where it is asserted, or state it once with the command that produces it. Nine figures are derived in section 0 and re-run at ab5debc, and the criteria that assert counts now name section 0 as the source of theirs rather than restating a number. One trap is recorded there for the next reader: grepping `internal/verb/definition.go` for the literal `Guide: \"references\"` answers 16 rather than 18, because `get` and `set` write the constant `referencesGuide`, so section 5.1's check reads the parameter's field rather than the source text."
---
Every figure in this spec is derived by a command written beside it, and section 0 is the table of those derivations.