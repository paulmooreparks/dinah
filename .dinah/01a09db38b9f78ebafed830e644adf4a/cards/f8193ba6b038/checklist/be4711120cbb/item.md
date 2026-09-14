---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:30Z
ordinal: 6
note: Re-verified independently on Test cycle 2026-08-27, on the re-merged tree at a50b68c. `go test ./cmd/dinah/ -run TestAPreVerbRefusalDecodesTheSameUnderBothMachineForms -v -count=1` passes, env cleared. Manually reproduced a pre-verb refusal under --format compact (claim of a nonexistent card, both after a bad --actor and as a plain unknown-card refusal) and confirmed the refusal/detail/affordances match the --format json equivalent byte for byte after normalizing the record framing.
---
A pre-verb refusal reached under --format compact (an unresolvable --actor value, or any other refusal main.go's reportError raises before a verb runs) and a dinah.ambiguous-workbench refusal reached under --format compact both decode to the same Refusal, Detail, Context and, for the ambiguous case, Workbenches values that --format json produces for the same invocation.