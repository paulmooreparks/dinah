---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:09Z
ordinal: 7
note: "This is the card's own reason for existing, so it is checked over MCP rather than at a terminal: the asymmetry the parent named is that an agent could not change one word of prose anywhere, and a criterion proving it at a terminal proves the half that was never broken. Both subject sets derive from `Field.Prose`, and the fatal on an empty set stops a build where nothing is marked prose from passing for free. Plant that reddens it: fill the event's `To` from the value in the prose arm rather than leaving it empty. It compiles, the write succeeds, and the run reports a `to` member on a line that must carry none."
---
Every prose field is writable over MCP, carries its line breaks into the anchor, and puts no copy of itself in the journal, while every field that is not prose refuses a value carrying a line break. `internal/verb/fields_prose_test.go`, `TestProseTravelsToTheAnchorAndNotToTheJournal`: for each name where `bench.FieldOf(kind, name).Prose` is true, a `set_field` call carrying a three-line value succeeds, `get_field` returns the same three lines, the entity's anchor body equals them, and the journal line the write appended carries `field` and carries neither `from` nor `to`. For each name where `Prose` is false, a `set` carrying an embedded newline refuses `malformed` naming the field, and the same value with the newline removed is accepted. The run is fatal when either subject set is empty.