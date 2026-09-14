---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:01Z
ordinal: 12
note: "Test-stage re-verification via independent Python sweep of en.json for the bare word \"level\": exactly 9 raw occurrences matching the criterion's list key for key; 2 context-excluded (unknown-level, unknown-depth), 7 actually checked. `go test -run TestATranslationUsesTheDeclaredWord ./internal/msg/` passes on the merged tree, including the check.card.4 backfill (confirmed rendering \"der Wert ist eine Stufe, die dieses Feld deklariert\" via `dinah help card --lang de` against a throwaway workbench)."
---
The glossary's "level" term triggers on exactly the 9 keys where the English word "level" appears (check.add.5, check.card.4, param.add.priority.summary, param.add.severity.summary, param.card.value.summary, refusal.dinah.unknown-level, check.contents.2, check.tree.4, refusal.dinah.unknown-depth), all referring to the same declared-ordered-set concept with no second sense to carve out; TestATranslationUsesTheDeclaredWord passes against the corrected corpus (one German entry, check.card.4, backfilled to contain "Stufe"; Hindi already carries स्तर on all nine and needs no fix).