---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:41Z
ordinal: 15
note: "Armed by changing bench.StorageFormat from 2 to 3. Red: \"docs/quick-start.md:328 declares format: 2 and the file the binary wrote carries format: 3; either the document is stale or testdata\\quickstart-file-blocks.txt:19 declares that the block teaches the key\". The same run also reddened the two `dinah version` transcripts, which is the replay doing its own job on a changed constant. This is dinah-446's own arming re-run against the replacement, so with AC-14 it pays for removing TestTheQuickStartWorkbenchFileDeclaresWhatTheBinaryWrites and checkAnchorBlockDeclaresTheBinarysValues. Constant restored, green."
---
A tool-owned frontmatter value is caught from the binary side. Arming: change `bench.StorageFormat` at `internal/bench/bench.go:76` from 2 to 3, run `go test ./cmd/dinah -run TestTheQuickStartMatchesTheTool`, and watch it fail naming `docs/quick-start.md`, the `format` line inside the block that opens at 326, the document's `2`, and the file's `3`. Restore the constant and watch the run go green.