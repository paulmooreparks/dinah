---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:46Z
ordinal: 4
note: Compares the returned array against an expected array of keys, in order, in a pure test with no editor. The fourth case is the one that carries the operator's ruling, and the assertion on it is that the result's length is the selection's length plus one and its first entry is the invoked row. A regression narrowing the union to the selection alone fails that length assertion rather than passing quietly.
---
`targetsFor` is covered in all four cases: no element and no selection answers the empty array; an element with no selection answers that element alone; an element whose key is in the selection answers the selection with no duplicate and in the selection's order; an element whose key is not in the selection answers that element followed by the whole selection.