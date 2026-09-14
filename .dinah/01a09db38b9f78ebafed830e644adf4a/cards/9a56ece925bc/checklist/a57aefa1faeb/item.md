---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:17Z
ordinal: 2
note: "Verified at 28323a1. TestEveryReferenceShapeEditAcceptsNamesAFile calls bench.ResolveEditTarget on all 47 generated shapes, requires os.Stat(answer).Mode().IsRegular() for the 24 declared opening and a *contract.Refusal whose Name equals the declared name for the 23 declared refusing. Armed by plant 2 (unconditional contract.Refuse at the top of ResolveEditTarget): exactly 24 shapes reddened with \"is required to open a file, and the resolver refused\", no refusing shape complained, and the run reported --- FAIL rather than a build failure."
---
Every reference shape the grammar admits opens a regular file or refuses by name. `TestEveryReferenceShapeEditAcceptsNamesAFile` in `cmd/dinah/edit_reference_sweep_test.go` calls `bench.ResolveEditTarget` on every shape `editReferenceShapes` generates and, for a shape declared `opens`, requires no error and requires `os.Stat(answer).Mode().IsRegular()` to be true; for a shape declared refusing, it requires a `*contract.Refusal` whose `Name` equals the shape's own declared refusal name. The accepting and the refusing halves are both asserted in the same run, so code refusing everything fails.