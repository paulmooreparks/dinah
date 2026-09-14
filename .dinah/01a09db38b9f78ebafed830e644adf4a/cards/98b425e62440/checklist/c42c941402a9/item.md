---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:16:35Z
ordinal: 4
note: "Direct run: `dinah --lang de --nosuchflag --lang hi` exits 2, renders in Hindi (not German), confirming last-occurrence-wins."
---
dinah --lang de --nosuchflag --lang hi exits 2 and renders in Hindi, not German: the last complete --lang found across the whole argument list wins, matching the last-occurrence-wins rule a successful parse already applies to a repeated flag.