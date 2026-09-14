---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:09Z
ordinal: 2
note: "The replacement half is what stops this passing against a build that refuses everything: a tree with `get` broken fails the second column of every row. Plant that reddens it: leave the `get` arm in `runWorkbench`'s switch. The retired row then exits 0 and the run reports that `dinah workbench get title` was expected to refuse `dinah.usage` and did not. A false failure would come from asserting the refusal sentence rather than the name, so the assertion reads the first whitespace-separated token of stderr and never the prose after it."
---
Each of the six retired spellings refuses with the name spec section 7.1 gives it, and the generic invocation that replaces it succeeds in the same run. `cmd/dinah/retired_spellings_test.go`, `TestTheSixRetiredSpellingsRefuseAndTheirReplacementsWork`: a table of six rows, each carrying the retired invocation, the refusal name expected on stderr's first token, and the replacement invocation with the value it must print. `card get` and `card set` expect `dinah.unknown-command`; `workbench get`, `workbench set`, `workstream get` and `workstream set` expect `dinah.usage`. Every retired row must exit 2 and every replacement row must exit 0 and print the expected value. Over MCP, `tools/list` carries no tool named `card`, carries `get_field` and `set_field`, and a `workbench` or `workstream` call naming action `get` or `set` is refused while the same read through `get_field` answers.