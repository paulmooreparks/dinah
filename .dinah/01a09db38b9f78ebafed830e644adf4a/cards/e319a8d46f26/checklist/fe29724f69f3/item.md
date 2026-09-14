---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:50Z
ordinal: 38
note: "Two reasons, both checked against the tree at 4c33c2c. A workstream cannot be selected at all, because `TreeElement` in editors/vscode/src/tree.ts is root, note, column, group, card, attachmentsGroup and attachment, with no workstream member. A column carries two refusals a card does not, `dinah.occupied` and `dinah.last-column`, which appear in the declared-precondition table in internal/verb/checks.go, and turning either into a sentence a reader acts on is separate work. That table is cited here for what it is, a declaration of preconditions rather than the verb's refusal order; D-11 records the correction, since `Library.Archive` in internal/verb/beyond.go refuses on conditions the table does not list.\n\nNothing is built in a way that has to be torn up, and round 4 restates that in terms of the shape §6 now declares. `archiveCard` takes one `CommandContext` and does nothing else, so it is a sibling of `claimCard` and serves a column context as readily as a card one. The confirmation is a separate export, `askArchiveConfirmation`, which is the table entry's `ask`, so a later card offering Archive on a column row writes the sentence a column needs, declares its own row resolution, and reuses the verb and the whole bulk layer unchanged. Round 3 made the same claim about a function taking a list of contexts, which is the filtered shape finding 1 rejected; the claim survives the repair because what a later card reuses is the per-row verb rather than the loop."
---
Archive is offered on card rows only in this card, and on no other row kind.