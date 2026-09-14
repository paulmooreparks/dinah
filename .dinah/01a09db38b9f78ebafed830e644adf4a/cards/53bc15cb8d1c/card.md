---
title: Resolving interrupted acts reports nothing about what it resolved
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
The checker can finish acts that an earlier run left interrupted. When it does, it says nothing about what it resolved, in either the terminal output or the machine-readable output. A person runs it, something changes, and nothing tells them what.

This is close to a defect fixed on dinah-49 but not the same one. There, the report already carried the detail and only the terminal renderer forgot to print it, so a single branch fixed it. Here nothing records what was resolved in the first place, so the report needs the field before anything can print it.

Two reviewers reached that conclusion independently while working dinah-49 and both judged it out of that card's scope.
