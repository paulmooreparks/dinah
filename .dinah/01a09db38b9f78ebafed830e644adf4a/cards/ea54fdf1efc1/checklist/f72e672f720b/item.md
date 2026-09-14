---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:12Z
ordinal: 43
note: "The card's own level write already held this rule and a guard in cmd/dinah asserts it, so the merged writer had to keep it for a card. Keeping it for a card alone would have meant a per-kind branch inside the writer, which is the shape this card exists to remove, so it holds for every kind.\n\nThat is a behaviour change on the workbench and the workstream, which journalled a line for a write that changed nothing. The spec does not name it either way. An append-only journal recording an act that did not happen is the worse of the two, and one existing test noticed: the workbench write test wrote its operator field to the value it already carried, and now writes it to a different one and moves the actor with it."
---
A write storing the value the entity already carries succeeds, writes nothing and journals nothing, on every kind rather than on a card alone.