---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 4
note: Re-verified at Test by running the built binary. `dinah set wb1-1/criteria/1 column bogus-value` exited 2 with "unknown-column this workbench declares no column bogus-value...". Item anchor sha1 before and after the refused call were identical (62b0360502eb1471c9b17441581bc350cb401089), and `dinah get` still reported the prior value f23d3cb83d48 unchanged.
---
Running the built binary: dinah set <item> column bogus-value (a value bench.ColumnByRef cannot resolve) refuses unknown-column, and the item's anchor file is byte-identical before and after the call.