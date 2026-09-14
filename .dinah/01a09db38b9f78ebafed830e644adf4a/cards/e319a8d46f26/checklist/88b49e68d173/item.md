---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:50Z
ordinal: 44
note: "Multi-select makes a five-row drag possible, and `dragPayloadFor` in src/dragAndDrop.ts reads `source[0]` and nothing else, so a five-row drag would move one of the five, which is the ruling failing where it is most visible. The reviewer confirmed both halves by tracing the chain: `offerDrag` wraps that single payload and `applyDropVerdict` moves that one card. So drag is in scope. The silence rule preserves today's behaviour, stated in classifyDrop's own comment: a drop back onto the card's own column and a drop on a row naming no column both do nothing and show nothing, and a drag that missed still has to look like a drag that missed. A summary appears only when at least one payload was acted on or refused. D-14 settles what the drag's own selected count is, and D-15 declares the one shipped behaviour this change alters."
---
A multi-row drag moves every dragged card, and a drop whose every verdict is `ignore` stays silent.