---
title: No translated help row can silently collide with the column beside it
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The help screens lay a command's preconditions out in columns with a fixed boundary, and a row whose text reaches that boundary runs into the column beside it with no gap. Nothing in the repository catches it. The risk is not hypothetical: an English line landed exactly on the boundary once and was caught only because a reviewer counted characters by hand. Every language the tool gains makes it likelier, because a translator has no way to know the limit exists and the tool never complains.
