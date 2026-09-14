---
title: Build Queue
slug: build-queue
kind: work
operator_owned: false
---
The waiting buffer for cards that have cleared the design half, including the operator's station there, but that nobody has started building. It absorbs the shuffling of priority between review and implementation.

Cards here are first in line. The tier on the card says which sessions may take it: a session working at a given tier takes cards at that tier or below.

### Who takes a card from here, and where it goes

Any session whose own tier satisfies the card's, highest priority first. Taking a card from here moves it into Implement.

Every card reaches this queue the same way, from Operator Design Review, because this workbench runs one route and every card walks all of it.

### Whether a card is ready to be taken

A card here may be waiting on another card. Read its links with `dinah show <card>` and leave a card alone while the card it is parked behind is still open. There is no command that answers "is this card gated" in advance, and there does not need to be one: the hold refuses the move when you attempt it and names itself in the refusal, so attempt the move and read the answer rather than asking first.

### When to leave a card here

- The tier does not match the sessions currently running, and it is waiting for a capable one to arrive.
- Higher-priority cards in flight are consuming the capacity at Implement.
- It is deliberately parked behind another card.

### A card must never sit here waiting on the operator

Work waiting on a person does not belong in a queue that advertises itself as ready to build, because the next agent along will take it and find it cannot proceed. A card in this queue has already passed the design half's operator station, so a question surfacing now is one of two kinds, and what separates them is whether implementation can start without the answer.

**Implementation can proceed despite the question.** File it as an open question with `--owner operator` and `--column operator-code-review`, put the recommendation and the tradeoffs in the note, and leave the card here. It rides forward with the card and is answered at Operator Code Review, where the hold will stop the card because the item names that column. An item naming a column the card has already passed holds nothing, which is why the column matters as much as the owner.

**Implementation cannot start without the answer.** The card is not ready to build, whatever column it sits in. Move it back to Agent Design Review, claim it, and block it there with `dinah block <card> "<the question>" --kind operator-ruling`. That block frees the claim you just took, and nothing follows it.

### This column holds nothing

A queue is a place to wait, so this column declares no hold and no reject target.
