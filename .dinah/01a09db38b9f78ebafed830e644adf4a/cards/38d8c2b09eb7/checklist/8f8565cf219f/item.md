---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:39Z
ordinal: 7
note: This is AC-1 made checkable on a diff. The reviewer reads the added lines of one diff rather than re-reading four documents, and the second clause names the specific paragraph where the temptation to correct all three figures is strongest.
---
`git diff origin/main -- docs/quick-start.md internal/guide/guides/ editors/vscode/README.md` introduces no spelled-out or numeric count of tools, commands, query fields, or member names anywhere in the added lines, and the MCP paragraph at docs/quick-start.md states no tool count, no command count, and no list of shell-only command names.