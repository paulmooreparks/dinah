---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:21Z
ordinal: 18
note: "`internal/mcp/tools.go:442` publishes the same sentence in the tool schema that the terminal prints, and `Param.SummaryKey`'s doc comment says that is deliberate. A clause appended in `cmd/dinah/help.go` alone would leave an agent reading the narrow answer, so `ArgumentMeaning` moves into `internal/verb` and takes the caller's renderer. The ten sentences that enumerate kinds are rewritten to say what the argument is; the three that name a sub-kind (rename, cite, the shared item) keep their prose, because the table is too coarse to hold a sub-kind and a rendered clause cannot say it."
---
The clause is composed in one shared function both heads call, and ten argument sentences give up the enumeration rather than keeping a second copy of it. D-12 does the same for six command summaries.