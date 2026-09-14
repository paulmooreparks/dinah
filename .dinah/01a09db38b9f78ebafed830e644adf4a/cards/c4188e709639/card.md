---
title: a board using the block workaround has no way to find its own workaround blocks
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
dinah-201 adds `awaiting_outside`, a state declaration saying the workbench waits there on somebody outside the flow. Before it, the only way to keep such a card out of the ready queue was to block it, which overloads the block with an impediment it does not have.

dinah-201 deliberately ships no automatic migration, and that call is right. CORE-BLOCK-5 forbids restricting a block's reason to a closed set, so nothing distinguishes a workaround block from a genuine one except free prose, and a migration that guessed would unblock a genuinely blocked card. Refusing to guess is correct.

What is missing is the other half. A board that used the workaround and was never told about the change has no way to find its own workaround blocks except by reading every block reason by hand. The card's migration path, one line in a file plus one `unblock` per card, is only actionable by somebody who already knows which cards to unblock.

Agent Design Review named the shape that helps and guesses at nothing: a `check` advisory naming any state that declares `awaiting_outside` and still holds blocked cards. That is a structural fact rather than an inference about intent. It cannot mistake a real block for a workaround, because it asserts nothing about why the card is blocked; it says only that a station now able to express waiting still has cards parked in the older way, which is exactly the set worth a human's eye. dinah-201 considers a `check` row only for an unrelated done-state case, so this one was never weighed there.

The spec should settle whether this is a `check` advisory or a finding, what it says when the state declares the flag but the blocks are genuine (it must not read as an error, since a waiting station can hold a legitimately blocked card), and whether it survives after every board has migrated or retires itself. Worth asking too whether the same advisory has value on a board that never used the workaround, since the answer decides if this is a migration aid with an end date or a standing check.

Sequence after dinah-201, which it depends on entirely: without `awaiting_outside` there is nothing for the advisory to key on.
