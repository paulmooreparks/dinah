---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:43Z
ordinal: 10
note: "Verified 2026-09-12 on head 995636e: go test ./cmd/dinah -run TestAnUnmigratedWorkbenchReadsAndRefusesToAllocate passed in 0.18s over the workbench declaring format 2 with numbers in frontmatter. dinah show fx-1 resolved the card and printed its reference, dinah ls printed the same references it prints today, and dinah add refused with dinah.needs-number-migration whose next step names dinah check --migrate-numbers --yes."
---
An unmigrated workbench reads and refuses to allocate. `go test ./cmd/dinah -run TestAnUnmigratedWorkbenchReadsAndRefusesToAllocate` passes over a workbench declaring `format: 2` with numbers in frontmatter: `dinah show <slug>-1` resolves the card and prints its reference, `dinah ls` prints the same references it prints today, and `dinah add` refuses with `dinah.needs-number-migration` whose next step names `dinah check --migrate-numbers --yes`.