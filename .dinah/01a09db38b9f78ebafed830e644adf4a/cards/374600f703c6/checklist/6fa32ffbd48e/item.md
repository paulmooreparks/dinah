---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:36Z
ordinal: 16
note: "dinah-433 wrote the ReadDir-then-Stat discrimination into ListWorkbenchIDs and then had to warn, in that function's own doc comment, that listIdentifiers seven files away carries the opposite idiom and must not be copied. A rule that has to be restated as a warning beside every copy is a rule with no single home. Putting it in one function means the next collection listing inherits it instead of being warned about it. Nothing on this card then enforces that a new reader calls it: the guard that would have done so left with D-4 on 2026-09-11 and is now dinah-486, and section 8 says so. What this card does enforce is narrower and it is AC-2, which holds the seven callers of readCollection by name inside internal/bench.\n\n[Corrected at Agent Design Review round six, 2026-09-11. The closing clause read \"and the guard decision enforces that nobody reads a directory any other way\", which rested on D-4 after D-4 went obsolete, so a live resolved decision on this card still argued from a decision the card no longer carries. Round six's search covered `whole-tree guard`, `examined set`, `rule 5`, `exemption`, `minFirstHalfSites` and `TestEveryDirectoryReadAnswersItsFailure`; the bare phrase \"the guard decision\" matches none of those, which is the workbench's own rule that searching a document for a phrase finds copies of the phrase rather than copies of the claim. Nothing above this bracket is otherwise changed.]"
---
One shared reader, readCollection, performs the read and the existence discrimination, and ListIDs, ListWorkbenchIDs and listIdentifiers all call it. The platform reasoning is written once rather than three times.