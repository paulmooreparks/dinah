---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 7
note: "PASS. A tools/call of `status` with an empty arguments object, against a server started inside a workbench with no --root, answers with that workbench's own root. Shown to track the starting directory rather than being constant by repeating the identical call from a second workbench and getting the second one back. One ambiguity recorded rather than resolved: the returned `root` is the .dinah/<id> store directory rather than the directory named to `dinah init`. That is Dinah's own noun for the workbench directory, and `dinah workbenches` prints the same path, so behaviour is unchanged either way."
---
A `tools/call` of `status` carrying no `workbench` property, sent to `dinah mcp` started inside a workbench, answers with a status whose `root` member is that workbench's directory.