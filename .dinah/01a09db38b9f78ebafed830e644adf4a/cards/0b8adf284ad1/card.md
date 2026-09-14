---
title: A wrapped row indents past the edge of a narrow terminal
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
When a value outruns its column, the row breaks and the remainder is indented to line up with the column it belongs to. That indent is measured against the declared column widths and knows nothing about how wide the terminal actually is. On a narrow window the indent alone can exceed the available width, so the continuation arrives as a run of blank space with a fragment floating after it.

The behaviour predates the card that generalised the wrapping to a whole row, since the earlier single-column form had the same property. Cascading several columns can push the indent deeper and reach the problem sooner.

The card decides whether the tool should read the terminal's width and adapt, or whether a declared width is the contract and a narrow window is the reader's problem.
