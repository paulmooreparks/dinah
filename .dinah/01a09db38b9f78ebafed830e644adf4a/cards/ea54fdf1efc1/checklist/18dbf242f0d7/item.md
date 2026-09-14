---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:12Z
ordinal: 40
note: "Round 2's fourth major asked for the consequence to be weighed again now that dinah-450 gives the field teeth at move time. I weighed it by running it rather than by reading it, against a workbench declaring gate_items on its work column.\n\nWhat I measured. `dinah file <card> open_question <text> --column 319ee962375f` holds the card out of that column: the move refuses `unresolved-item`. The same file with `--column doing`, the column's own slug, holds nothing: the move succeeds. And on a card already held, `dinah set <item> column doing` lifts the hold silently and the move succeeds.\n\nThe rule the guard table is generated from survives, so the field stays unguarded: a field's write guard is the guard its create path applies, `Library.File` stores `req.Column` with no resolution and no check, and guarding the rewrite alone would make a value legal to file and illegal to correct.\n\nWhat changed is where the defect lives. It is not that `set` writes an unvalidated value; it is that `file --column` accepts a slug the gate can never match, because `bench.GatingItems` compares against the column's identifier. That predates this card, it reaches every item filed by a slug, and it wants its own card resolving the reference the way `add --column` already does. Filing that card is outside this one's scope and is named in the handoff."
---
An item's `column` stays unvalidated after dinah-450, and the defect the re-derivation turned up belongs to `dinah file` rather than to this card.