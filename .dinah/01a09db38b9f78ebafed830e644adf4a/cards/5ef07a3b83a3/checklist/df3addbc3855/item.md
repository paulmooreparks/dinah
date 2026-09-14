---
kind: acceptance_criterion
state: pending
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:43Z
ordinal: 8
---
The workstreams and their memberships crossed. `dinah workstream --format json` is run against the real workbench from `c:\Users\paul\source\repos\dinah`, letting discovery find it rather than naming a workbench directory, and reports a set of workstreams whose slugs are a superset of the 21 the Andoneer board declared at the cutover instant: addressing, dogfood, structured-judgements, vsix, token-cost, beta, beta-wave-1, beta-wave-2, beta-wave-3, beta-wave-4, beta-wave-5, how-work-moves, state-model, levels, integrity, terminal, handoff, conformance, authoring, model-gaps, spinout. The run prints the cardinality of both sets and names any slug present on one side and not the other. Separately, for every open card that carried a workstream membership on Andoneer at the cutover instant, the same membership exists on the Dinah side, checked by comparing the two membership sets card by card and printing how many cards were compared; a comparison over zero cards fails rather than passing. A failing run is a missing workstream, a missing membership, or a compared-card count of zero. This fails today: the Dinah workbench declares no workstreams at all.