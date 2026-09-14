---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:43Z
ordinal: 8
note: "Verified 2026-09-12 on head 995636e: go test ./internal/bench -run TestCheckReportsTheRegistryDefects passed within the 41-test -run match, over one fixture per condition. The six findings (duplicate, repeated, missing, stranded, malformed, in-frontmatter) each fired with their key, path, and detail, a clean migrated workbench reported none of the six, an archived card whose anchor will not load was reported rather than passed over, and a workbench whose cards carry no line drew exactly one check.card-number-missing per card with no duplicate."
---
Each of the six findings fires on exactly the state it names. `go test ./internal/bench -run TestCheckReportsTheRegistryDefects` passes over one fixture per condition, asserting the key, the path, and the detail of each, that a clean migrated workbench reports none of the six, and that the two properties dinah-439 paid for survive: an archived card whose anchor will not load is reported rather than passed over, and a workbench whose cards carry no registry line draws exactly one `check.card-number-missing` per card and no duplicate report.