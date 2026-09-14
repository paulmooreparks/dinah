---
title: The attach command documents the flag it already accepts
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The command that attaches a file accepts a description flag and writes what it is given into the attachment's own record, but its help text names neither the flag nor the field. So a person can only find it by reading the source or by being told, and anything generated from the command's own declaration has the same blind spot. This was found while measuring what the tool writes for another card, which needed the flag in order to produce a shape it had to sample.
