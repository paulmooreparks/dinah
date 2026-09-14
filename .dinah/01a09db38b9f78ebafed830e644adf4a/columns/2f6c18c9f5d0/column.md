---
title: Design Queue
slug: design-queue
kind: work
operator_owned: false
---
The Spec station's waiting buffer: classified cards waiting for spec-stage attention. Raw capture lives upstream in Intake; by the time a card sits here it has been through Triage and carries a tier, a severity, a priority and a workstream.

### Who takes a card from here, and where it goes

Spec-stage agents and the operator. Arrival in this queue is already the go signal from Triage, so a card may be taken freely: highest priority first, and a tier the session can actually work.

Taking a card from here moves it into Spec. Nothing else happens in this column.

### When to leave a card here

- The tier does not match the sessions currently running.
- The card is deliberately parked behind another card; read the incoming links with `dinah show <card>`.
- Spec is already full, and the queue absorbs the wait so the station does not.

Queue age is the station's demand signal. If this queue grows while Spec sits idle, what is missing is somebody to work Spec rather than work to do.

### A card parked here on purpose

When two cards would edit the same document, leave the second here until the first lands, and say on it with `dinah comment` why it is waiting and what it is waiting for. Waiting here costs nothing, and it is cheaper than a conflict in a file whose figures are derived from each other.

### This column holds nothing

A queue is a place to wait, not a place where anything is answered, so this column declares no hold.
