---
title: A tree in a window too narrow to hold it loses its shape
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A table too narrow for its columns falls back to the stacked form, one block per row, and a tree drawn that way stops reading as a tree: the guide characters travel as the value of a field and the reader reassembles the nesting from them. dinah-151 met this while drawing its tree at forty columns. The card carries a second and older finding with it, that the stacked fallback fires only when a column stands at its own heading and a field overruns it, so a table whose first column measures wider than its own heading never stacks however narrow the window gets. dinah-151's tree no longer reaches that case, because the heading it settled on is long enough to fire the fallback, and another table can. Reproduce both against whatever dinah-132 leaves behind before speccing either, since dinah-132 owns that predicate and was in Acceptance on it when this card was filed.
