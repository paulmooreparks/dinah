---
kind: acceptance_criterion
state: verified
ts: 2026-09-14T02:17:41Z
ordinal: 20
note: "Armed. Red: \"testdata\\guide-blocks.txt:44: the entry expects a block to open at internal/guide/guides/mcp.md:196, and no fence opens there\". Line 196 carries the `for ref in <ref> ...` shell line rather than a fence, so the entry became stale rather than pointing at another block. Restored, green. Nine checks are new on this card and all nine now carry an arming."
---
A stale guide block entry is caught. Arming: repoint the entry for `internal/guide/guides/mcp.md:195` at line 196, run `go test ./cmd/dinah -run TestNoGuideBlockEntryIsStale`, and watch it fail naming the guide, line 196, and the entry that expects an opening fence there. Restore the line number and watch the run go green.