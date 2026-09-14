---
title: A rule and its summary line are checked against each other
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
The contract document states each rule twice: once as the rule itself, and once as a one-line summary in the index. Nothing compares the two, so they can say different things indefinitely. That is not hypothetical: a rule published this week capped an exception at a single item while its own summary line described the exception as unbounded, and the divergence survived a full review pass because no reader thought to hold one against the other and no test could. The rule text freezes on publication, so a divergence found later costs a retirement rather than an edit.
