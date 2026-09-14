---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:08Z
ordinal: 28
note: "New precondition row. German \"die Referenz nennt eine Art, die Anhänge führt\" and Hindi \"संदर्भ ऐसी किस्म को नामता है जो संलग्नक रखती है\". The German follows the shape of the neighbouring rows check.archive.1 and check.card.1, both of which open \"die Referenz nennt\". The Hindi verb नामता है is the form check.attach.2 and check.card.1 already use for \"names\". Width was the constraint worth checking: on `dinah help attach` at COLUMNS=80 the refusal column widens to the 20 columns of dinah.not-attachable, which leaves 49 for the middle column, and TestChecksColumnNeverGluesInAnyLanguage renders every row of every shipped locale at 52 runes and passes. Source stamped with msg.Fingerprint of the English."
---
`check.attach.3`: written fresh in German and Hindi against the current English and its context.