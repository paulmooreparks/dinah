---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:25Z
ordinal: 21
note: Agent Design Review's major finding was that none of verb.ReferenceKindOrder's six values is honest for member-position, member-identifier or member-name, and requiring one would have written a false statement into the very roster this card builds to stop false statements about addressing. The reason none fits is that ReferenceKind is the vocabulary of what a command's reference may name, per its own doc comment, and a selector names a step within a reference rather than a thing a reference names. `collection` is wrong because the reference names one member rather than the collection, and `below-card` is wrong because a member's holder need not be a card, as wb/attachments/1 shows. So the field is left empty and a test asserts it is empty, which makes the absence a stated property rather than an omission. The head forms' kinds are asserted as a set in both directions against {workbench, workstream, column, card}, which also records that below-card and collection are named by no head form because a reference reaches both by composing a head form with segments.
---
A selector form carries no reference kind, and the roster splits into head forms that carry one and selector forms that do not.