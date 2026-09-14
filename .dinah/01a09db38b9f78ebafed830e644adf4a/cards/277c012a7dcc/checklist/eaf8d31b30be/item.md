---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:19Z
ordinal: 5
note: "Verified. `go test ./cmd/dinah/ -run TestNoReferenceKindLabelCarriesTheClauseSeparator -v -count=1` passes on the shipped catalogues and logs \"48 labels read\" (cmd/dinah/reference_kinds_help_test.go, the t.Logf followed by the t.Fatalf count guard). The shipped passing run is the accepting side pinned beside the plant, so a guard refusing every label would fail it rather than satisfy it. Plant: writing \"eine Spalte; genauer gesagt\" into de's reference.kind.column reddened at the t.Errorf naming de and that key, \"de's reference.kind.column reads \\\"eine Spalte; genauer gesagt\\\", which carries the clause separator \\\"; \\\", so the guard over the rendered clause would read it as two labels\", with the count still 48 so the failure is a real read rather than a short sweep. Restored byte-identically and green. The ban stays on the six reference.kind.* labels alone: attach's and attachments' summaries carry \"; \" legitimately and the shipped run passes with them."
---
`go test ./cmd/dinah/ -run TestNoReferenceKindLabelCarriesTheClauseSeparator -v` passes on the shipped catalogues and logs forty-eight labels read; writing `"; "` into `de`'s `reference.kind.column` reddens naming `de` and that key.