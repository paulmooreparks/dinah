---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:41Z
ordinal: 13
note: "Armed three times, each restored before the next. (1) A dollar-sign line written into the commands block: \"internal/guide/guides/getting-started.md:51: the block declares commands and this line opens with a command prompt, which is the shape the quick start's replay drives and nothing drives here; write the command without its leading dollar sign\", which is the retired rule's own message. (2) A bare output line with no dinah: \"the block declares commands and this line is not one, so the block shows output nothing drives:\" followed by the line. That is gap two's defect in its evadable spelling. (3) A closing brace deleted from the json block at mcp.md:23: \"the block declares json and does not parse: unexpected end of JSON input\". Restored, green."
---
A block declaring the wrong class is caught. Arming, three runs of `go test ./cmd/dinah -run TestEveryGuideBlockShowsWhatItDeclares`, each restored before the next: write `$ dinah next` into the `shows=commands` block at `internal/guide/guides/getting-started.md:49` and watch it fail with the message the retired rule gave authors; write a bare output line carrying no `dinah` into the same block and watch it fail saying the block declares commands and this line is not one; delete a closing brace from the `shows=json` block at `internal/guide/guides/mcp.md:23` and watch it fail naming the JSON error.