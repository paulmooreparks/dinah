---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:50Z
ordinal: 40
note: The alternative is one prompt per card, which for twelve cards is the shape a reader abandons halfway, and abandoning halfway is exactly the partial result this card exists to make visible. The intersection is keyed by `move.column`, which `movePick` in src/cardCommands.ts already establishes as the value the move verb takes. The per-card `dinah instructions` calls that feed the intersection are reads, so a reader who cancels the single prompt has changed nothing on the board. An empty intersection shows `dialog.move.noSharedDestination` and prompts for nothing; the single-card path keeps `dialog.move.noLegalMoves` unchanged.
---
Move asks once, over the destinations every selected card will accept.