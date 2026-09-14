---
title: A guard that remembers line numbers goes stale on the next edit
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
The checks that make sure every table is registered and every uncovered block is accounted for identify their targets by file and line number. Inserting a single comment above one function was enough to break the registry and produce four findings for code nobody had touched, which means every future edit to that file carries a chore of retyping numbers, and a person under time pressure will retype them wrongly or widen an exemption to make the noise stop. The guard is the thing keeping a whole class of rendering defect out of the tool, so it needs to survive ordinary editing rather than punishing it.
