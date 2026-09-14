---
title: Lanes, so that some cards travel a different path than others
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
workstreams:
  - b3f924406e4c
  - 2cdb633015c1
links:
  - kind: parked_behind
    to: 5ef07a3b83a3
---
A board cannot say that some cards travel a different path through the flow than others. That bit a trial port where three of seven states were traversed by some cards and skipped by others, and the routing had nowhere to live. The operator has ruled this deferrable for the work he is tracking now, since every card on his current board travels the same seven states. It is filed so the need is on the record rather than rediscovered by the next board.

This card was split on 2026-08-24. It originally carried a second need, that a state be able to say it is a queue somebody pulls from rather than a station where work happens, and that half is now its own card. The two were separated because the deferral above only covers this one. Lanes are about routing, and a board whose cards all take the same path genuinely does not need them. Whether work is taken up at a station is a different axis, one the format already models with `awaiting_outside`, and an outside implementer hit it before anybody hit lanes.
