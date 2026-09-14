---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:27Z
ordinal: 7
note: "This is the criterion that holds D-2 to the code rather than to the spec's prose: the claim is that nothing of the reader's is written, and a claim about a whole tree is produced by a command over the tree, not by reading the module that was open. The sweep reads every `.ts` under `src/` and asserts the count it read is greater than zero, so a glob that matched nothing fails rather than passing. PLANT: add `const p = \".vscode/mcp.json\";` to `mcpServers.ts` and the sweep must go red naming that file and that line. This criterion is weaker than it looks and says so: it catches the spellings named above and cannot catch an obfuscated path composition, so it is a guard against the change somebody would actually write rather than a proof of the negative."
---
A repository-wide sweep asserts that no file under `editors/vscode/src/` names `mcp.json`, calls `workspace.getConfiguration(...).update`, or writes to any path under `.vscode/`, and reports how many files it read.