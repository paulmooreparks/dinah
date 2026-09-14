---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:41Z
ordinal: 11
note: "Armed twice, and this is the criterion that pays for removing TestNoGuideCarriesATranscriptTheReplayDoesNotDrive. With the dollar sign, red: \"internal/guide/guides/query.md:137 opens a fenced block and testdata\\guide-blocks.txt carries no entry for it; declare what the block shows\". Without it, the same failure with the same wording at the same line, which is the case the retired rule passed in silence. Block removed, green."
---
An undeclared guide transcript is caught whether or not it carries a dollar sign. Arming, twice: append to `internal/guide/guides/query.md` a fenced block whose body is `$ dinah query column:done` followed by an output line, run `go test ./cmd/dinah -run TestEveryGuideBlockIsDeclared`, and watch it fail naming the guide, the block's opening line, and `cmd/dinah/testdata/guide-blocks.txt`. Then remove the leading `$ ` and re-run, and watch it fail the same way. Remove the block and watch the run go green.