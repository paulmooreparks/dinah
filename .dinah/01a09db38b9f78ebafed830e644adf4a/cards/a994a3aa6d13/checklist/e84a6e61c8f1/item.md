---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 9
note: PASS. Against one server bound to a root holding two workbenches, a status call naming the second in its workbench property answers with that second workbench's root. Both workbenches were asked in the same session and each answered with its own. One registration reaching two workbenches is the card's whole point, and this is the call that shows it. The verifier noted that "the second workbench" could mean the second listed or the second created, verified both, and found the reading does not matter.
---
Against that same server, a `tools/call` of `status` naming the second workbench in its `workbench` property answers with a status whose `root` member is that second workbench's directory.