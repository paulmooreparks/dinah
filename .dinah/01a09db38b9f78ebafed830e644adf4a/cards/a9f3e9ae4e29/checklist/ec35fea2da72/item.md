---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:29Z
ordinal: 3
note: "Re-run: same test as AC-2 covers the off/clear path and passes."
---
`dinah set <column> hold off` against a column currently holding (gate_items: true on disk) removes the gate_items key entirely (not gate_items: false); `dinah get <column> hold` then returns "off", and a column that never had the key set also returns "off".