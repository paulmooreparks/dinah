---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:00Z
ordinal: 4
note: "Test-stage re-verification of the amended criterion (operator struck the second selector 2026-08-27). `go test -run TestContractTokensSurviveInBackticks -v ./internal/verb/` PASS on the merged tree. Confirmed the four token.* entries render naturally in context: `dinah help pull --lang de` shows the German owner phrase \"die Anfrage nennt einen Akteur\" and the shipped --override text correctly, no byte-identity violation attempted against token.* since that selector is out."
---
internal/verb/contract_tokens_test.go defines contractTokens() (composed from contract.Substate*, bench.WorkbenchAnchor, bench.LevelAxes, Commands(), each command's flag Params, and the literal "stdout") and literalTokens(), and TestContractTokensSurviveInBackticks fails an entry whose backtick-quoted contract token is missing, byte-identical, from a shipped non-English, non-skeleton translation.