---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:39Z
ordinal: 9
note: "Round 2. The previous revision replaced a false membership list with a false paraphrase of the same set, wrong about `reshape`, and no criterion on the card could catch it because every criterion looked for numbers. This one returns a verdict on the prose itself: one command named, no purposes described, no count, and the rule verified against the roster test rather than against a reading."
---
`go test ./internal/mcp/ -run TestEveryLibraryCommandIsServedOrExempted -v` passes and reports at least one test run, and the MCP paragraph in `git diff origin/main -- docs/quick-start.md` names no command in backticks except `guide`, describes what no individual command is for, and states no count, so the only claim it makes about the served set is the rule that test holds the head to.