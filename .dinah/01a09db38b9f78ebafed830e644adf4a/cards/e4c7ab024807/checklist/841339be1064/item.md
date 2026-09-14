---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:49Z
ordinal: 16
note: The card contributes one entry to an existing context menu and one modal whose chrome, button placement and dismissal VS Code owns entirely, so a drawn mockup would depict the editor rather than anything this card decides. The integration suite under editors/vscode/test/integration/ starts a real editor host, and the workbench forbids running it against the operator's editor or installing into it. src/registrationGuard.ts's own header already states that no test in this repository settles whether VS Code accepted a registration or whether the item reaches the menu a person right-clicks, so an integration test here would buy confidence it cannot earn.
---
No mockup and no integration test ship with this card.