---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:10Z
ordinal: 22
note: "dinah-456 section 5.3 fixes `func FieldsOf(kind string) []string` and says in the same breath that `FieldsOf` is the only place the distinction between a frontmatter key and the anchor's prose is recorded. A slice of names cannot record it, so the two sentences cannot both be honoured by one function. Keeping the signature and adding a sibling reader over the same `map[string][]Field` honours both: the declaration is in one file, the parent's callers compile unchanged, and the writer asks `FieldOf` for the four properties it needs. A `Field` value also carries the clearability and the guard, which are the other two facts that would otherwise become switch statements in `internal/verb` that nothing can sweep."
---
`bench.FieldOf` is added beside `FieldsOf` rather than changing the parent's signature, and both read one table so the prose distinction is recorded where the parent says it is recorded.