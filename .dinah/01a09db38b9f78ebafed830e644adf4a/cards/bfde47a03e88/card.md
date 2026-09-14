---
title: The contract's own stated row count is checked rather than asserted
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The contract document ends its index with a sentence stating how many rows it carries. That sentence is ordinary prose and no test reads it, so it can drift the moment somebody adds or retires a rule without updating the line. The count is correct today, and it stays correct only for as long as everyone remembers. Every neighbouring fact in that document is machine-checked, which makes this one the odd exception rather than a deliberate choice.
