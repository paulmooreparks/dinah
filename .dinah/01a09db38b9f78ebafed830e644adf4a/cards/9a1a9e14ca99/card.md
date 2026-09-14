---
title: The zero-result backstop stands down for a refusal that had nothing to do with the search
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The rename sweep has a backstop that refuses to report a clean zero when the raw diff plainly carries the rename it could not read. That backstop stands down whenever the run declined anything at all, anywhere in the range, on the assumption that the reader has already been told something went unexamined. The assumption is too broad. A block too large to align, in a file nobody was renaming, silences the backstop for a rename in a different file entirely.

What the reader sees in that case is a zero, no groups, and a line about a file they were not searching. Nothing connects the refusal to the term that went unread.

The narrower repair is to scope the suppression to the run that was actually declined rather than to the whole invocation. A refusal in one file would then stop speaking for a rename in another, and the backstop would keep working everywhere it does today.

This does not close the hole and it is not meant to. When the oversized run is itself the one carrying the rename, the current behaviour is the better of the two, because the reader gets a report naming the file and the line instead of a bare refusal naming nothing. An existing test pins that case deliberately. So the published limit still has to describe an exception after this card lands, and the card should say which exception survives rather than deleting the paragraph.

Filed out of the fourth code review of dinah-311. The reviewer weighed this against the repair that card took and judged both legitimate, with this one worth its own card rather than a fifth round there. The counterexample corpus entry written against the same code sanctions both. See dinah-311's comments for the reasoning on each side.
