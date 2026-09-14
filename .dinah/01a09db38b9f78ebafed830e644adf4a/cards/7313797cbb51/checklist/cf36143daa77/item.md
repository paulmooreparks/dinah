---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:25Z
ordinal: 12
note: "Established by running rather than by reasoning: `dinah path wb-f9a4ac64513f` against a live card whose identifier is f9a4ac64513f raises unknown-card, while `dinah path f9a4ac64513f` opens the card. splitRef cuts at the last dash and parses the tail with strconv.Atoi, so the identifier can never be the tail. Accepting it would give one card two composed spellings that have to be kept equal everywhere, which is the failure AddressedInItsOwnRight's own doc comment records for the cards collection. The guide's sentence was always about a collection member, so narrowing its subject is a correction rather than a retreat."
---
The spelling the guide implies and the tool refuses is `wb-<identifier>`, and the guide stops implying it rather than the tool starting to accept it.