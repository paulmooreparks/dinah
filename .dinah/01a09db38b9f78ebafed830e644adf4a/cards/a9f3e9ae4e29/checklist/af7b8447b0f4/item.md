---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
owner: holder
ts: 2026-09-14T02:18:30Z
ordinal: 11
note: "Reproduced from scratch at internal/verb/mutate.go:401 (if false && destination.GateItems): built, ran go test -run TestTheHold* -> only TestTheHoldCommandTurnsTheGateOnAndOff went red (\"the move into the held station succeeded after the hold was turned on\"); the 5 storage-level TestTheHold* cases stayed green. Restored file, git diff --stat empty, rebuilt, re-ran green. Also confirmed live against the built binary: move into held column refused exit 2, succeeded exit 0 after hold off."
---
End-to-end: after `dinah set <column> hold on` (via this new command, not a hand-edit), a `dinah move` of a card carrying an unresolved item naming that column into that column is refused unresolved-item, exactly as dinah-450's existing gate does for a hand-edited gate_items: true; and after `dinah set <column> hold off`, the same move succeeds. This is the one criterion that proves the new command actually turns the hold on and off rather than only writing and reading the stored value back.