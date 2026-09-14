---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:41Z
ordinal: 14
note: "Re-driven independently by Test in two fresh worktrees, since this is the card's central claim. On a detached checkout of cf651ab, changing line 328 from `format: 2` to `format: 1` and running `go test ./cmd/dinah -run TestTheQuickStartMatchesTheTool` exits 0: the previous tree does not catch it. On 56db064 the identical one-character change fails with \"..\\..\\docs\\quick-start.md:328 declares format: 1 and the file the binary wrote carries format: 2; either the document is stale or testdata\\quickstart-file-blocks.txt:19 declares that the block teaches the key\", naming the document, the format line inside the block that opens at 326, the document's 1 and the file's 2 as the criterion requires. Both trees restored and green. This reproduces Implement's own arming rather than relying on it."
---
The replay no longer passes on a document that agrees only with itself, and the previous tree is shown not to have caught it. Arming, in two trees: on a checkout of `cf651ab` with `TestTheQuickStartWorkbenchFileDeclaresWhatTheBinaryWrites` skipped, change `format: 2` to `format: 1` inside the `file` block that opens at `docs/quick-start.md:326` and run `go test ./cmd/dinah -run TestTheQuickStartMatchesTheTool`, and record that it passes. On this card's tree, make the same change and run the same command, and watch it fail naming the document, the `format` line inside that block, the document's `1`, and the file's `2`. Restore both trees.