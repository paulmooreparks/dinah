---
kind: decision
state: resolved
ts: 2026-09-14T02:17:39Z
ordinal: 14
note: docs/quick-start.md:367-371 sits inside the console fence opening at line 359 (the earlier citation of 364 was wrong; 364 is output text), it is absent from cmd/dinah/testdata/quickstart-exempt.txt, and it carries a wip_limit exactly as the guide's example does, so it is the closest replayed shape to what internal/guide/guides/principles.md:30 is drawing. It is a `dinah status` block rather than a `dinah columns` one, and it is authoritative for the guide's listing anyway because both heads print through renderColumns (cmd/dinah/render.go:256, reached from commands.go:550 and render.go:204); AC-10 holds that to a run of `dinah columns` rather than to this reasoning. Copying from it takes AC-1's second form. Copying from a fresh run would take none of the three, since nothing would afterwards hold the guide to the tool.
---
Finding 5's authority is the quick start's replayed block, not a fresh run of `dinah columns`.