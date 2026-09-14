---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:38Z
ordinal: 32
note: No. Shipped as "this card carries the item {detail}, which is not resolved, and this column holds until it is". docs/design/format.md:456 defines "station" as a column where an owner takes work up and "queue column" as one where nobody does, and a column declaring awaiting_outside is a queue column whatever its kind. D-9 names "AwaitingOutside combined with Hold carrying out or both" as the review-station shape this card exists to enable, so the spec's own prescribed sentence would be false in exactly the case the card was written for. Changed the noun to "column", which is the entity the refusal is actually about and the glossary word the German and Hindi catalogs already carry a form for. Nothing else about the sentence moved.
---
The spec prescribes the exit refusal sentence "...and this station holds until it is". Does the word "station" survive into the shipped catalog text?