---
title: Triage
slug: triage
kind: work
operator_owned: false
---
The classifying station. Every card entering this workbench stops here first, and nothing leaves Intake by any other door.

Think of it the way a hospital does. Most arrivals need a glance and a direction, a few need one look and an immediate answer, and nothing is admitted without somebody deciding where it goes. The glance is cheap precisely because it is a glance.

That cheapness is the discipline. A card arriving with tier, severity and priority already stamped is confirmed rather than re-derived: read it, agree or correct, move it on. Spending frontier reasoning on a classification that is already obvious is the waste this column exists to avoid.

### Where the card goes next

This workbench runs one route and every card walks it: Intake, Triage, Design Queue, Spec, Agent Design Review, Operator Design Review, Build Queue, Implement, Agent Code Review, Operator Code Review, Test, Merge, Acceptance, Done. There is no second path and no routing field, so Triage chooses a card's classification rather than its road.

Work needing a written contract goes to Design Queue, which is where every card goes. A card that turns out not to be ripe goes back to Intake.

Say in the move note when you expect the card to need the operator: a card that will produce a command transcript, an external interface, a change to how this workbench itself works, or published copy is going to stop at Operator Design Review before anything commits to it. You are not the last word, because Spec knows for certain once it has produced an artifact or not, and a predicted stop that proves unnecessary costs the operator one glance.

### The tier

Write the tier with `dinah set <card> tier <value>` and say why in one sentence. Default to the cheapest tier that can plausibly do the work. Raise it only for a concrete judgement need the cheaper tier cannot meet.

### The workstream

Read the set with `dinah workstream` and attach the card with `dinah join <card> <workstream>`, to more than one where the work genuinely spans them. A card belonging to no workstream is invisible to every workstream-scoped read from here on, so leaving it unattached quietly removes it from the views work actually gets planned from. When nothing fits, say so in the move note rather than inventing a workstream to fill the field.

### Severity and priority

Set both with `dinah set <card> severity <value>` and `dinah set <card> priority <value>`, and set priority in context rather than in isolation. Severity asks how bad the thing is on its own terms, and the card alone usually answers it. Priority asks when the card should be worked, which the card alone cannot answer, because that answer is a claim about this card relative to everything already in flight.

So read the workbench before you stamp. Scan the columns holding work in flight with `dinah ls <column>` rather than sweeping the whole workbench, and scan those rather than Intake and Design Queue, which are backlogs where a stale top-priority stamp costs nothing.

The top priority is a cap of one. If two cards are both at the top, neither of them is, and the field has stopped carrying signal for everybody who reads it. When a card genuinely displaces the incumbent, demote the incumbent in the same sitting and name both in the move note. When it does not displace the incumbent, stamp the rank below and leave the incumbent alone. A second top stamp written to avoid making that call is how a priority field decays into decoration. When the cap is already blown, one triage pass cannot restore it: do not add to the pile, say in the move note that the cap is over so the count stays visible, and leave the repair to a pass across the whole set.

Check for duplicates while you are here, against work already in flight: `dinah link <this card> <the live card> supersedes` and archive this one, or merge the content into the live card.

### This column holds nothing

Triage files items and routes cards; every item it files is settled at a station further along. So this column declares no hold.

### Claim before you classify

Cards arrive by being moved out of Intake, and that move leaves the card standing here ready rather than held. Claim it with `dinah claim <card>` before your first edit, because a card the workbench shows as ready is a card anybody may take out from under you. Batch the sitting where you can: take several cards, classify the set, move them all.
