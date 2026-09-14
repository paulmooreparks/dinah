---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:07Z
ordinal: 21
note: "`Workstream.Ref` at internal/bench/workstream.go:96 answers with the slug, not with `workstream/<slug>`, so the refusal detail for a workstream reads `probe-stream` where the caller typed `workstream/probe-stream`. `dinah.not-renamable` already prints the same bare slug, confirmed by `dinah rename workstream/probe-stream x.txt --json` answering `\"detail\": \"probe-stream\"`. Changing Ref would move printed output on every surface that names a workstream, including listings and headers, which is a wider change than this card should carry and one no criterion here would cover. Following the house form keeps one spelling across the two refusals rather than inventing a second."
---
`EntityRef.Ref` for a workstream stays the bare slug, so the refusal prints a spelling that does not resolve on its own.