---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:50Z
ordinal: 41
note: VS Code's TreeViewOptions.canSelectMany declares that argument one is the item the command was executed on and argument two is an array containing all selected tree items. It does not declare that the executed-on item appears in that array, and nothing here rests on behaviour the declaration does not carry. Taking the union is the only reading that cannot silently drop the row the reader aimed at, which is the operator's ruling in its worst form. Deduplication is by a key the caller supplies rather than by object identity, because nothing documents that the editor hands back the same object in both arguments.
---
The target set is the union of the invoked row and the selection argument, with the invoked row first when it is absent from the selection.