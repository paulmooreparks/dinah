---
kind: decision
state: resolved
ts: 2026-09-14T02:17:39Z
ordinal: 12
note: AC-1 offers three forms. The third is closed because internal/bench/bench.go:1546 refuses a workbench anchor declaring no `profile`, so the pair cannot be elided from the file block. The second is closed because the export transcript at line 1412 prints the value the file block supplied, so it is an echo rather than a witness; that loop is exactly what let dinah-core/0.9 survive four revisions. Only the first form is left, so TestTheQuickStartWorkbenchFileDeclaresWhatTheBinaryWrites is what the card's own rule requires and is what "no test changes beyond what finding 4 forces" permits.
---
Finding 4 forces a new guard, and that guard is in scope for this card.