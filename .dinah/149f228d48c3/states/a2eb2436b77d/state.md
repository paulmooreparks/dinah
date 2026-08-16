---
title: Triage
kind: work
operator_owned: false
---
The routing station. **Every card entering this board stops here first, and nothing leaves Intake by any other door.** The point is not the tier stamp, it is the route. Triage is the only place a card's path through the board is chosen, and every stage after it inherits that choice without revisiting it. A card that skips this column does not get "no route", it gets the default one, which is the longest and most expensive on the board.

Think of it the way a hospital does. Most arrivals need a glance and a direction, a few need one look and an immediate answer, and nothing is admitted without somebody deciding where it goes. The glance is cheap precisely because it is a glance.

That cheapness is the discipline. A card arriving with tier, severity and priority already stamped is confirmed rather than re-derived: read it, agree or correct, choose the route, move it on. Spending frontier reasoning on a classification that is already obvious is the waste this column exists to avoid, which is why it runs at the workhorse tier by directive.

### The route

Five decisions, and they travel together.

**Does this card need a spec?** Triage asks the question and never answers it, because writing the spec is Spec's work. Work needing a contract goes to Design Queue, which is the normal path. A mechanical fix whose contract is already obvious from the card alone goes straight to Build Queue, meaning nothing touching schema, auth, a published contract, or more than one behavioural surface. A card that turns out not to be ripe goes back to Intake.

**Will the operator have to approve something?** Call it here, as a prediction rather than a finding. A card that will produce a UX sketch (a command transcript or --help text), an external interface, a change to how the board itself works, or published copy is going to stop at Operator Design Review before anything commits to it. Say so in the move-note so the route is visible from the start. You are not the last word: Spec knows for certain once it has produced an artifact or not, and the operator stations are structural stages on the lanes that run them, so a predicted stop that proves unnecessary costs the operator one glance.

**The lane.** Read the current set with `list_lanes` rather than trusting any count written down elsewhere, including here. Lanes are data and this prose is not, so when the two disagree the data wins. Stamp the choice with `set_card_lane`, so every later stage reads the route off the card instead of inferring it from column position. Leaving the field empty is a routing decision rather than a deferral of one: the card takes the board's default lane, which is the longest.

**The tier.** Stamp `expected_tier` with a one-sentence reason. The set is tiny, minimal, workhorse, frontier and apex. Default to the cheapest that can plausibly do the work, which is minimal for narrow mechanical cards and workhorse for well-specified ones with clear acceptance criteria. Promote to frontier only for a concrete judgment need the cheaper tier cannot meet, or for one of the hard-frontier overrides in the workbench instructions.

**The workstream.** Read the set with `list_workstreams` and attach the card with `add_card_to_workstream`, to more than one where the work genuinely spans them. A card belonging to no workstream is invisible to every workstream-scoped sweep, report and filter from here on, so leaving it unattached quietly removes it from the views work actually gets planned from. When nothing fits, say so in the move-note rather than inventing a workstream to fill the field.

### Severity and priority

Set both, and set priority in context rather than in isolation. Severity asks how bad the thing is on its own terms, and the card alone usually answers it. Priority asks when the card should be worked, which the card alone cannot answer, because that answer is a claim about this card relative to everything already in flight.

So read the board before you stamp. No read surface filters by priority, and a whole-workbench `list_cards_brief` is the wrong tool once a board carries real volume. Scan by column instead, passing `column_id`, and scan the columns holding work in flight rather than Intake and Design Queue, which are backlogs where a stale top-priority stamp costs nothing.

**The top priority is a cap of one.** If two cards are both `now`, neither of them is, and the field has stopped carrying signal for every agent and every pull surface that reads it. When a card genuinely displaces the incumbent, demote the incumbent in the same sitting and name both in the move-note. When it does not displace the incumbent, stamp `next` and leave the incumbent alone. A second `now` stamped to avoid making that call is how a priority field decays into decoration. When the cap is already blown, one triage pass cannot restore it: do not add to the pile, say in the move-note that the cap is over so the count stays visible, and leave the repair to an operator pass across the whole set.

A card arriving with severity and priority already set is confirmed rather than re-derived, and the board check above applies to that confirmation exactly as it does to a fresh stamp.

Duplicate-check while you are here, against work already in flight: link `duplicates` and archive, or merge the content into the live card.

### Who works it

`card-triage` works this column, dispatched like any other station. Its mandate is the routing decision above and nothing else, so the card leaves as soon as the five calls are made.

Cards arrive by a pull from Intake, by an agent or by the operator, and that pull commits the puller to the triage glance rather than to the work. It is the only way in, and it is the only way out of Intake. A card promoted from Intake to any other column has skipped its route, and the right correction is to send it back there rather than to guess the route downstream.

Batch the sitting where you can: pull several cards from Intake, classify the set, route them all. Claim while classifying, because the dark-work rule covers operators too.
