---
title: Batch checklist operations in one invocation
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
workstreams:
  - de90dc7a5ac4
  - fdfdeaaff2dd
---
A test-stage agent resolving five acceptance criteria should not pay five CLI invocations, five lock acquisitions, and five journal round-trips. Backlog.md lets one command check, uncheck, and remove multiple criteria by index in a single call, and that grammar is worth adopting for checklist-item state changes generally. This is CLI and MCP surface over the existing checklist entities; each item's lifecycle event still lands in the journal individually. The interesting design question is atomicity, meaning whether a batch is one transaction or a sequence that can partially succeed.
