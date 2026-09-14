---
kind: open_question
state: resolved
column: aa6cd1c6ae5f
owner: operator
ts: 2026-09-14T02:16:32Z
ordinal: 22
note: "Answered on this card's own timeline by two self-corrections from the triage agent: dinah-124 is about the human rendering (the identifier a person reads), not the machine payload, so it shares no surface with this card beyond the word \"identifier\"; dinah-128 is an independent bug (guide ignoring a requested machine form) that needs no new format and waits on nothing. So the answer is three separate cards, not one absorbing the other two. The predecessor card proposed in the same thread (collapse 24 branch sites into one projection point) is also unnecessary: 22 of the 24 sites already call the same nine-line `emitJSON` function in cmd/dinah/render.go, so the projection point already exists and there is nothing to collapse first. This card's spec proceeds directly, defining a compact encoding over the existing single projection point."
---
Are dinah-31 (token-lean output), dinah-124 (internal identifiers on request) and dinah-128 (guide command answers in machine form) one card, one card that absorbs the other two, or three separate cards, given all three sit on the same incomplete projection surface? And does whichever card(s) result wait behind a predecessor that first collapses the 24 sites currently branching human-vs-machine output into one projection point, since adding a compact third form over today's shape would turn each into a three-way branch instead?