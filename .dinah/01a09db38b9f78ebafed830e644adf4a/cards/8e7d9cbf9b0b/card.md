---
title: Generate the request from the verb table instead of checking it by hand
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Follow-on from dinah-282. That card ships a guard (three test layers) that checks whether each head's argument wiring matches the verb table's param declarations. The guard is necessary but it is a checker sitting beside two hand-written mappings: internal/mcp's request2Args/assignValue/assignMarker switch on each param's literal name, and cmd/dinah's run<Name> functions (plus the shared (*session).request helper) read parsed values by hand.

dinah-282's spec adds a `Field` to `verb.Param` naming the exact verb.Request field each param fills. Once that declaration exists, request2Args needs no hand-written switch at all: it can set the named field by reflection directly, for every command, mechanically. dinah-282's own Layer-2 MCP test already performs this exact reflection to verify the switch is correct, which is the proof that generating it is no harder than checking it.

Scope this card to the MCP side first (replace assignValue/assignMarker/request2Args's per-param switches with reflection over the declared Field, using the same mapping dinah-282's guard reads). The CLI side, where each run<Name> hand-assigns whatever the shared (*session).request helper does not cover, is a larger, second-phase change and can be scoped once the MCP side is proven.

Doing this does not replace dinah-282's guard: even generated code needs a test asserting the generation is exhaustive over verb.Commands()/verb.Params(), so the guard's Layer 1 (roster coverage) stays regardless of how Layer 2's gap closes.
