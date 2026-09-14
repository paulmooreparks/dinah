---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:12Z
ordinal: 4
note: "Confirmed at the command: the doing station in my fixture drew both ready (5) and active (0) groups even while active held no card, matching TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty's bufferDoing half, green."
---
The same tree still draws ready and active under a station whether or not either is occupied, unchanged from dinah-275's tier 3. Test hook: the bufferDoing half of TestAColumnThatTakesNoWorkUpDrawsNoStateGroupWhenEmpty.