---
title: The reader never checks that a card's condition is a condition
column: 5ea2db0272fc
state: ready
severity: major
priority: next
tier: frontier
---
Found by the fourth code review of dinah-287 on 2026-08-27, and filed rather than fixed there because it is older than that card and independent of it.

A card carries a column identifier and a condition. Nothing on the read path checks that the condition is one of the values a condition may take. So a card whose condition field holds a twelve-hex identifier is listed happily, and `dinah ls` prints that identifier under the Standing heading at exit 0:

```
  Card  Standing      Title
  fx-3  a44374b0615a  card three
```

`dinah check` catches it, using a predicate the reader does not have. So the tool already knows the value is wrong; the read path simply never asks.

## Why this is not dinah-287's

That card added a guard refusing a card left in the retired vocabulary, keyed on whether the card carries the new column field. A card carrying the column field has been converted, so the guard passes it, correctly. The output above is reached by hand-editing a converted card, which is a different fault from the one that card exists to catch.

The reviewer scored it a major rather than a blocker for that reason, and recommended a follow-up. Three earlier rounds on that card had already spent themselves on the vocabulary question, and a fifth would have bought a fix to something the rename neither introduced nor worsened.

**One thing on dinah-287 does need correcting there, and is not this card's.** The guard's comment says the absence of the column key "is the whole question". It is not, and a comment claiming more than its code does is the shape this board has spent a week removing.

## What this card has to settle

**What the reader does when the value is wrong.** Refusing is the obvious answer and it is not the only one, because a reader that refuses turns a cosmetic corruption into an unopenable workbench. The alternative is to render the condition as unknown and let `check` carry the finding, which keeps the workbench readable and still tells somebody. Decide it, and decide it for the whole read path rather than one command.

**Where the check lives.** `check` already has the predicate. A second copy in the reader is the duplicated-rule shape this board keeps paying for, so the reader should consult the same predicate rather than grow its own.

**What it costs.** A card's condition is read on every listing of every card, so a per-card validation sits on the hottest read path there is. Establish the cost on a workbench with many cards before choosing where the check goes, because the answer may be that it belongs once per open rather than once per card.

**Whether the same hole exists for the column identifier itself.** The condition is the field the reviewer demonstrated. A card whose column identifier names no column is the mirror image, and nobody has checked whether the reader validates that either. Enumerate what the reader validates and what it takes on trust, by reading rather than assuming, and say which other fields are in the same position.

## Related

dinah-287 renames the two fields and ships the vocabulary guard this was found beside. `dinah check`'s existing predicate is the thing to reuse rather than re-derive.
