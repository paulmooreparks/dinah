---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:17:44Z
ordinal: 20
note: "Reproduced against a scratch copy of the real workbench on 2026-09-13 rather than reasoned about. `dinah add` on the workbench as it stands is refused with `dinah.needs-number-migration`, naming `dinah check --migrate-numbers --yes`. Running that first is silently wrong: it stamps `format: 3`, which is at or above `ContainerFormat` (internal/bench/bench.go:91), which makes the containment rule bind, which this workbench fails because its 12-hex directory name is not a workbench identifier under `IsWorkbenchID`. The next command is then refused with `dinah.needs-container-migration`, and the container migration stamps `format: 2`, discarding the first migration's only effect. Container first, numbers second, and the container migration previews without `--yes` (exit 5, \"Nothing was written. This repair moves directories\") so it is run that way first."
---
The container migration runs before the number migration, and both run before any card is created.