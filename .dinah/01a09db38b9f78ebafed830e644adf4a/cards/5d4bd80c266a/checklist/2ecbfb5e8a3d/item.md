---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:46Z
ordinal: 14
note: "No, not soundly, and no code attempts it. The trap needs two facts: an item names a gated column, AND that column is where convention says the item gets resolved. Dinah's core has a field for the first and none for the second (owner is free text describing who, never where; item resolution itself is unrestricted by column or claim). A structural refusal built only on \"item names a gated column\" would refuse the ordinary, intended case (an AC verified during Test, naming Merge as its gate) exactly as often as the deadlock. Mitigation is a workbench documentation practice (name the column after the one that answers the item, never the one that does), not tool enforcement."
---
Can the tool detect the "item assigns work to the step it holds" self-deadlock trap and refuse the configuration?