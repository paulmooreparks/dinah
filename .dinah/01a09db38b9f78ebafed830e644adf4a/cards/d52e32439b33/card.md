---
title: Distributed workbench synchronization
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
workstreams:
  - fdfdeaaff2dd
---
Some dinah workbenches will live outside any git repository, and people will still want the same board visible from more than one place. The question is what synchronizes board state between instances: the operator is loathe to recreate git, suspects a state-sync message in the API might suffice, and fears the general problem gets ugly. The design has to say what happens to claims and moves when two copies of a workbench exist, which is a different problem from syncing the card text itself.
