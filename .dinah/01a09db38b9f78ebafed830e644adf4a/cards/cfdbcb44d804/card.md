---
title: check reports a citation that names nothing
column: 5ea2db0272fc
state: ready
severity: major
priority: next
tier: workhorse
workstreams:
  - b3f924406e4c
  - de90dc7a5ac4
  - 4fd7a9f0b8ff
---
dinah-240 specifies the citation a checklist item carries and names the findings a checker owes, and it deliberately writes no code, because nothing in Dinah creates a checklist item and a finding written against an unreachable path would be unreachable too. This card is where those findings become real.

What it owes, once the checklist verbs exist: `check.unknown-scheme` for a citation whose scheme the workbench's `evidence:` block never declared, and `check.dangling-citation` for a citation whose scheme carries a `resolves:` collection and whose target names nothing in it. The format fixes both, including that a card-scoped collection resolves against the citing item's own card and that a scheme carrying no `resolves:` key is beyond what the tool can see, so the checker reports nothing about its target.

The work is scoped rather than open. `internal/bench/check.go` already declares the finding catalog and already performs both walks these findings need, since `check.dangling-link` resolves an id against the cards collection and `check.dangling-workstream` does the same against workstreams. What is new is reading the workbench's declaration and the item's field, and choosing the walk from the declaration rather than from the scheme's name.

Two dependencies decide when this can start. Nothing creates a checklist item today, so the entity has to exist before a check over it can. dinah-240 must land first, since it is the document these findings are read from.

A third finding may join them depending on how dinah-240 clears code review, covering a citation whose scheme demands the before-and-after observation and carries none. That condition is specified and its name is not yet minted.
