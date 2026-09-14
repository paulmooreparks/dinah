---
title: Nothing shows what has been archived, so an archived card can only be reached by somebody who already knows its reference
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
workstreams:
  - 58f3e3eb621a
links:
  - kind: spawned_from
    to: e319a8d46f26
---
Filed on 2026-09-13 at the operator's direction, alongside dinah-490, which gives the extension an archive command and deliberately ships without this.

## The gap

`dinah restore <ref>` puts an archived entity back, and dinah-461 established that an archived entity has an address again. What is missing is any way to find out what is in the archive. Checked on the tool as it stands: neither `dinah list_cards` nor `dinah tree` takes an archived flag or mentions the archive, and the extension's tree shows live rows only.

So restore is reachable only by somebody who already knows the reference of the thing they archived. That is fine for an agent acting on a card it just handled. It is not fine for a person who archived the wrong row.

dinah-490 makes this sharper rather than creating it, because it puts an archive command in front of a human with a mouse, and a misclick there currently has no route back through any surface the person was using.

## What this card owes

A way to see what has been archived, and to put a chosen one back. Whether that is a CLI listing the extension renders, a flag on an existing read, or a view of its own is the design question, and the answer decides how much of it is CLI work.

Prefer extending a read that already exists over minting a parallel one. `dinah-490`'s own framing notes that the card collection has a live half and an archived half and that the check that walks both already exists, so the archived half is not new ground.

## Two things already known, so the design does not rediscover them

The eight extension catalogues already carry `history.event.restored`, added for forward compatibility, and its context note records that no command writes a restore event today. That note stops being true when this card ships, so it is a line to correct rather than to leave.

`dinah check` walks both halves of the card collection and reports an archived card whose file it cannot read rather than skipping it, which dinah-439 established on the grounds that the archived half is where damage hides. A listing that quietly omits an archived card it cannot read would undo that reasoning on a different surface.

## Why it is its own card rather than part of dinah-490

The operator was offered both shapes on 2026-09-13 and chose to ship archive first with this filed immediately rather than held. It is filed now, in his words, so that the one-way door is visible on the board rather than remembered.
