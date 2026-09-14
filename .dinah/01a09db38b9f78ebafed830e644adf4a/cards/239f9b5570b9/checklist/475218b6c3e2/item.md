---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:50Z
ordinal: 7
note: "Verified on 3b9c51c. Test: internal/verb/search_ref_test.go, TestAWorkbenchHitIsAddressedAsTheWorkbench. Command: go test ./internal/verb -run TestAWorkbenchHitIsAddressedAsTheWorkbench. Plant: the workbench hit filled Ref from l.Bench.Slug again; built clean, and the run went red at search_ref_test.go:42 (\"the workbench hit is addressed \\\"fx\\\", wanted \\\"workbench\\\", which is the form path, show and edit accept\") and at search_ref_test.go:49 (\"which resolves to nothing: unknown-card: fx\"). Restored byte-identically (cmp clean) and green again. The second, standing assertion requires the bare slug to keep resolving to nothing, which is what stops this guard becoming a tautology if slug resolution ever widens."
---
A workbench that matches a search prints the spelling that reaches it. In internal/verb/search_ref_test.go, TestAWorkbenchHitIsAddressedAsTheWorkbench writes a phrase into the workbench anchor's body, runs Library.Search for it, and asserts the workbench hit's Ref equals bench.WorkbenchRef and that Bench.ResolvePath of that value returns the workbench anchor's path. A second assertion asserts Bench.ResolvePath of the workbench's slug alone returns an error, so the criterion records why the slug is wrong rather than only that it changed. Arm it by restoring `Ref: l.Bench.Slug` at the hit, which compiles and turns the first assertion red.