---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 2
note: Re-verified at Test by running the built binary against a scratch workbench (C:/dinah-scratch/dinah-474-test/bench/wb-1). `dinah file wb1-1 acceptance_criterion "held out of doing" --column doing` exited 0; `dinah show wb1-1 --json` then reported the item's column as 15a9235a6266, which `dinah columns --json` names as Doing's identifier, not the slug "doing" that was typed.
---
Running the built binary: dinah file <card> criterion "text" --column <a column's slug> succeeds, and reading the item's column back reports the column's identifier, not the slug that was typed. Prove by running the tool and comparing against dinah get <column> id.