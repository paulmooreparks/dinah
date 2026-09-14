---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:45Z
ordinal: 33
note: "Round 5 defeated the round 4 prototype twice. A helper declared in `bench.go`, which rule 2 never inspects, can take the card as an argument from a permitted function in an allowlisted file and hold it on the bench for an exported accessor, so the card leaves the function that read it without any of 2c's escape shapes occurring; passing it is the only step inside an allowlisted file, so the argument list is where the clause has to sit. Separately the prototype tracked `*ast.AssignStmt` alone, so `var c, err = LoadCard(...)` bound nothing and round 4's own defeat came back clean with three characters changed. `append` is excluded because the guard already follows its taint and firing on the argument as well would report one thing twice. The clause is affordable because of what this card leaves in the allowlisted files: `check.go` keeps one free reference in a readability probe with no reason to hand a card anywhere, `card.go`'s three readers are exempt by name, and the migration reads `card.Number` off a selector. On the tree at c1ae4a9 the clause fires once, at `check.go:288`, which is the card walk this card converts to `b.LoadCardIn`, and the same scan over that file with the line converted reports nothing under rule 2."
---
Rule 2c reads both of Go's binding forms and fails on a tracked card standing in a call's argument list, with the builtin `append` the one call excluded.