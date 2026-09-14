---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 2
note: "Re-verified independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c. Full `go test ./... -count=1` is green across all ten packages, env cleared. Directly compared bytes rather than trusting the suite: built the branch binary and trunk's binary (origin/main at a01014e) into separate throwaway workbenches, ran `--actor Tester --json ls` on each after filing one identical card, and diffed the two outputs after normalizing only the nondeterministic id/column/revision hex values -- zero diff. Confirms emitCanonical under formatJSON produces the same structure and field set as before this card."
---
Every existing test asserting the shape or content of --json output (compat_test.go and every other *_test.go under cmd/dinah/) passes unchanged after this card lands, byte for byte, demonstrating emitCanonical under formatJSON is identical to today's emitJSON under s.json=true.