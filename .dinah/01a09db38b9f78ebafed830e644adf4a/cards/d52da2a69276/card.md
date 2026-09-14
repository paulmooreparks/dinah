---
title: A refusal quoting what you typed does not let it break the line
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A refusal echoes the offending word back so the reader can see what the tool objected to, and it echoes it raw. A value carrying a newline therefore prints a refusal across two lines, and other control characters can do worse to a terminal. Nobody can type one at a shell, but the agent tool surface can send one, so this reaches the audience the product is built for. Since refusal composition was consolidated, the echo runs through the one composer every refusal uses, so the fix is one place and covers all of them. There is a second half that is a question about the published grammar rather than a defect: a tab separates two terms of a query and a newline does not, and the two should agree.
