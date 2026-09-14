---
title: "Bench lifecycle over MCP: init, extract, and discovery, or a ruled cut"
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The MCP server covers every work verb and every bench-bound read, but init, extract, and workbenches have no MCP counterpart because the server is bench-bound and lifecycle sits outside its world. The onboarding principle says an agent with the binary and the MCP wiring should build a rich workbench from the word go, and today it cannot create one over MCP at all. The card's job is to close the gap or name it: either lifecycle and discovery tools join the MCP surface with a story for what the server is bound to before a bench exists, or bench creation is ruled CLI-only and that cut is recorded where the tri-surface expectation is documented. The local conveniences path, edit, and config stay CLI-only either way and are not part of this question.
