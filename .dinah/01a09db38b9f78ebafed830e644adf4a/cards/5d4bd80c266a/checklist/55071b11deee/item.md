---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:46Z
ordinal: 10
note: "A single boolean, gate_items: true, no per-kind list. Kind-selectivity comes entirely from which items a workbench's own filing convention writes column: on; the tool never inspects Kind to decide whether to gate. Reuses the existing item.Column field, no new field."
---
Does a column declare which item kinds it holds, or does it declare only that it holds, with kind-selectivity left to filing convention?