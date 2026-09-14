---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:40Z
ordinal: 6
note: TestTheAmbiguousCardRefusalCarriesItsCandidatesAsData reads the --json report's context.cards and the MCP show tool's context.cards for the same reference, and both carry the newline-joined identifiers. No head change was needed; the existing Extra plumbing carries it.
---
The machine surface carries the candidates as data rather than only as rendered text. Against the CLI fixture in the spec, `runCLI(t, root, "--json", "show", "fx-1")` emits a refusal report whose `refusal` is `dinah.ambiguous-card` and whose `context.cards` holds the newline-joined identifiers, and the MCP head's response for the same reference carries the same `context` member.