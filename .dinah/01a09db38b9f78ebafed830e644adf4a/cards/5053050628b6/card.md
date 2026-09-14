---
title: A bounded listing says how deep it looked
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A root-scoped read walks eight directories deep when the caller names no depth, and a workbench past that bound is simply absent from the answer. The caller cannot tell a listing that was cut short from one that saw everything.

The bound itself is not hidden. The `max-depth` parameter's own summary reads "how many directories deep to look, counting down from the root; 8 when you name none, and 0 for no limit", so a caller reading the command's help learns both that an unqualified call is bounded and what the number is. What no caller can learn is whether the bound actually bit on the call they just made.

What this card asks for is that the five root-scoped answers and the positional `workbenches` form each carry the depth the walk applied, so a client can tell a bounded listing from an exhaustive one without any directory being reported that nobody examined. The case that matters is the default: the caller named no depth, a choice was made for them, and today nothing in the answer says so.

Reporting the skipped directories instead is the wrong remedy and was already ruled out. Decision D-4 on dinah-281 set the bound, and its criterion AC-4 requires that a workbench past the bound be absent rather than reported, on the grounds that describing a directory the walk never opened would be describing something nobody looked at. That ruling stands. This card changes what the answer says about itself, not what it says about unvisited directories.

Filed out of the second code review of dinah-281, where both the implementer and the reviewer independently judged it separate work rather than a defect in that card. It is separate because it adds a field to five published answer shapes, which is a design decision, and dinah-281 ships correct against its own approved design. The VS Code extension is the first client that will care, since a truncated tree drawn as a complete one is worse than a truncated tree that says it is one.
