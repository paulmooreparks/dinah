---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:52Z
ordinal: 24
note: "dinah-456 section 4.3 marks the columns listing unchanged, and the machine side agrees on its own evidence. A column's identifier is accepted in the head position of a reference: `dinah show 6c013653c662` answers the column at 22a35fc, as do its slug and its title, so a client holding ColumnView can name the column from what it already has. A comment is the contrasting case, because its identifier resolves only in the selector slot of a path that already names the card, which is why CommentView is owed a ref and ColumnView is not. ReshapeColumn, ReshapeRetirement and RemintReport report what a run did or would do rather than viewing an entity, and a reshape preview names a column that does not exist yet. All four are recorded in the machine-side guard's exemption list with a ground, so the reasoning is checked rather than remembered."
---
verb.ColumnView is left without a ref field, and the three reshape and remint reports are left alone.