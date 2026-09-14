---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:53Z
ordinal: 30
note: "The literal is written three times at 22a35fc, in IsWorkbenchRef, in Library.Attachments and in Library.rootOf, and this card adds a fourth use at the search hit, where l.Bench.Slug is published today and resolves to nothing. A fourth copy of a string three places already share is the shape a rename gets wrong, so the name lands with the fourth use rather than after it. dinah-456 D-10 already ruled that the workbench prints `workbench` for its own row and composes the addresses below it against the slug, so this changes a row to match a settled ruling rather than opening one. A fifth copy of the literal already exists and is deliberately not reused: bench.KindWorkbench at internal/bench/containment.go:9 is the entity kind the containment table keys on and answers what a thing is, where WorkbenchRef answers how the workbench is spelled in the reference grammar. The two agree today and are different questions, and collapsing them would tie the on-disk kind vocabulary to the address grammar, so a grammar that came to spell the workbench `bench` would force a rename on a kind the format stores."
---
bench.WorkbenchRef is minted for the literal "workbench", and the search hit prints it.