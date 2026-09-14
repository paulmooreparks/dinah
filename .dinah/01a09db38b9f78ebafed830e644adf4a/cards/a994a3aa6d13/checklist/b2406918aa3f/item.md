---
kind: decision
state: resolved
ts: 2026-09-14T02:16:51Z
ordinal: 34
note: "Section 5.1 of docs/spec/core-profile.md scopes the model to one workbench and says the profile says nothing about how one workbench relates to another. The section 10 boundary table rules out views across several workbenches at once, with the reopen condition that two tools need to agree how a card in one workbench refers to a card in another.\n\nNothing in this card meets that condition. Every tool call still answers about exactly one workbench. The `workbenches` listing names workbenches rather than relating cards. No response carries a reference from a card in one workbench to anything in another.\n\nThis restates the operator's own reasoning in D-1 rather than adding to it, and it is filed as a decision so the question is closed on the card rather than asked again at Implement or at Code Review.\n\nRepository claims verified for this decision: 2 (the text of section 5.1, and the boundary row and its reopen condition at line 1124 of docs/spec/core-profile.md)."
---
The core profile is not reopened by this card, and the spec says so where a later reader will look.