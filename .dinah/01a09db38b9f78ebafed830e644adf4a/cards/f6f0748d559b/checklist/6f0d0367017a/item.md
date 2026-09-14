---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:27Z
ordinal: 3
note: "The whole array is compared with `deepEqual` rather than checked for containing `--workbench`, because an argv that carries the flag after the command word parses differently and a containment check passes on it. PLANT: reorder to `[\"mcp\", \"--workbench\", root]` and the test must go red printing both arrays. SECOND PLANT: rewrite `command` inside `planMcpServers` to `PATH_NAME` or to any other composed value, and the verbatim assertion must go red, which is what holds D-9's \"the extension publishes the path the binary reported and composes no other\" to being true of the code rather than only of the prose. The `executable` the function is given is the absolute path `--json version` reported, never the bare name and never a path the extension built; AC-15 holds the caller to that and this criterion holds the function to passing it through untouched."
---
`mcpServers.test.ts` asserts that `planMcpServers` composes `args` as exactly `["--workbench", root, "mcp"]`, `command` as the `executable` string it was given verbatim, `cwd` as the root, and `version` as the tool version it was given, for a single target.