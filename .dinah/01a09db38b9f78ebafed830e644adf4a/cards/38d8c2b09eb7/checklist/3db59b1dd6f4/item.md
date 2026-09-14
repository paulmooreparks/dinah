---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:38Z
ordinal: 4
note: Finding 2. The grep proves the false sentence is gone rather than reworded around, and the run proves what replaced it is true of this build.
---
`grep -n "does not accept" internal/guide/guides/first-session.md` returns nothing, and `dinah show --help` run from a scratch directory under C:\dinah-scratch exits 0 and prints the command's help page, so the claim the guide used to make is refuted by the same binary that embeds the guide.