package main

// ambiguousFlow is a workbench definition one station wider than the one dinah
// init writes, so that two columns qualify as a bare pull's destination at once.
// The default flow cannot produce that: it runs intake, doing and done, and a
// pull lands no card at a column where no owner takes work up, so doing is the
// only destination a bare pull ever has.
const ambiguousFlow = `{
  "profile": "dinah-core/0.5",
  "title": "Wide flow",
  "columns": [
    { "id": "b00000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Doing", "slug": "doing", "kind": "work" },
    { "id": "b00000000003", "title": "Review", "slug": "review", "kind": "work" },
    { "id": "b00000000004", "title": "Done", "slug": "done", "kind": "done" }
  ]
}`
