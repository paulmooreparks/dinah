---
kind: open_question
state: resolved
column: aa6cd1c6ae5f
owner: operator
ts: 2026-09-14T02:16:54Z
ordinal: 22
note: Found by this card's sweep, not by the card itself. CORE-JSON-7 preserves an unrecognized member across the interchange form, so the wire is covered and the model is not. Dinah's own states really do carry more than the profile names, including lifecycle and the external-wait property that section 10's own row records as living outside the core. Adding the statements takes a minor increment and a changelog entry under DOC-VER-11 and DOC-CHG-2, which is why this card does not do it. Worth a card of its own.
---
Section 5.2 gives a state no analogue of CORE-CARD-8 and CORE-CARD-9: nothing says a state may carry fields the profile does not define, and nothing requires a tool to preserve one it does not recognize. Should a later revision add those two statements for states?