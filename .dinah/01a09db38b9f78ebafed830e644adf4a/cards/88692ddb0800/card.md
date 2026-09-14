---
title: The changes tool names a command an agent cannot call
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The `changes` tool on the MCP interface hands an agent a command name that is not a tool the agent can call. Following it dead-ends, which is the one thing the product's own affordance promise says must never happen.

It predates dinah-273 and that card did not touch it. It is the last place outside the single rule that card established, under which every card-shaped answer now asks one function what may be done rather than writing out its own list. This one still writes out a name, and the name belongs to a vocabulary the MCP surface does not serve.

Found by the code review of dinah-273 while looking for an eighth caller of the rule that card centralised. The reviewer recorded the pattern in the workbench's convention document rather than folding the fix into a card that had already been through three code-review passes.
