---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:33Z
ordinal: 24
note: The guide says "The listing says so on its own first line." renderTree at cmd/dinah/render.go:335 prints treeHeader first and the contents.archived line after it. Running the tool against a scratch workbench confirms the notice appears on the second line, under "a card with a comment (probe-1) contains 1 entities." The sentence becomes "The listing says so once, on the line under its heading.", and the proving test is tightened from strings.Contains to a line-position assertion so the sentence the guide makes and the sentence the test proves are the same one. This is a defect this card found, fixed on the card that found it rather than filed.
---
The fifth archive statement is corrected in this diff before it is pinned, because the shipped sentence is false.