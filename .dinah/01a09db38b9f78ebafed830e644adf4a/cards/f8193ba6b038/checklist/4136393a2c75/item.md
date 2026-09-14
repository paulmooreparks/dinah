---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 7
note: "Re-verified independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c. `go test ./cmd/dinah/ -run TestAShapeWithNoCompactRenderingEmitsTheCanonicalJSON -v -count=1` passes all ten subtests (columns, show, status, config, check, query, log, workbenches, version, workstream), env cleared. Manually ran `--format compact columns` against a scratch workbench: output was valid JSON, byte-identical to `--json columns` on the same workbench state."
---
A command whose answer is not one of the three compact-capable shapes (for example states, show, status, config, or check) run under --format compact produces output that is valid JSON, parses successfully, and is byte-identical to that same command's --format json output.