---
title: A search verb with structured hits
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
workstreams:
  - fdfdeaaff2dd
---
Agents and humans both need to find things in a bench without knowing which entity holds them. Grep works today, but it returns raw lines rather than entities, and an agent then burns tokens re-reading anchors to figure out what it found. A search verb in the library, projected to all heads like every other verb, would return structured hits that identify the entity and the matched field. Backlog.md ships fuzzy search across tasks, docs, and decisions, and it is one of that tool's most-used surfaces, so the demand is proven. This is tool surface, not contract; the profile is untouched.
