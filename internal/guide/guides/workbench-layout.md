# What a workbench looks like on disk

A workbench is readable without the tool, which is the point of keeping it in
text. This is what you will find.

```
<workbench>/
  workbench.md              the flow, the levels, the standing instructions
  journal.ndjson            workbench-scoped actions, once there are any
  states/<id>/state.md      one station: title, kind, limit, instructions
  cards/<id>/
    card.md                 the card: position, claim or block, framing prose
    journal.ndjson          this card's history, one JSON object per line
    comments/<id>/comment.md
    attachments/<id>/
      attachment.md         filename, description, provenance
      payload/<file>        the bytes, under their original name
  archive/cards/<id>/       cards taken out of the live set
```

Every card, state, comment, and attachment is a directory named by twelve hex
characters, made real by its anchor file. Identity therefore survives
renaming, and a directory without its anchor is garbage rather than a
half-built one.

A card carries its own position. States never list their members, so moving a
card is one write to one file, and an interrupted move can never strand a card
in two places at once.

The journal is append-only and its line order is its event order. A crash can
tear the last line and nothing before it. An action carries the names of
whatever it refers to as they stood at the time, so the history still reads
years later when the states it names have been renamed or removed.

Hand-editing is legal. Edit the frontmatter with an editor when you need to,
then run `dinah check`, which reports the defects the format forbids: a claim
without the substate that implies it, a block with no reason, a card naming a
state the workbench does not declare, a link pointing at no card, and a
position that disagrees with the journal.
