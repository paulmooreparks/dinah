---
title: Jira process wrapper as a domain layer
column: 5ea2db0272fc
state: ready
severity: minor
priority: later
workstreams:
  - fdfdeaaff2dd
---
GK runs an arcane Jira process, and the operator wants it wrapped in a yokoten workbench that models the honest method while Jira remains the corporate record. The shape agreed in design discussion: a one-way projection where bench transitions carry declared Jira obligations performed under the bench's approval gates, inbound Jira changes surface as flags rather than auto-moving cards, and the arcana are encoded once in the mapping instead of living in people's memories. It requires no Jira administrator and no workflow changes, which is what makes it adoptable inside GK. This is the likely second external proof after the first tester's Jira-resolution workbench, and its first cut needs no new core mechanism because the agent working the card performs the projections per state instructions; deterministic per-state hooks are a later candidate that this card's experience should inform.
