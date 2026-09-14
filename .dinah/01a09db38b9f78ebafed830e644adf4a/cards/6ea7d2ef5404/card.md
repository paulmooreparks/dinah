---
title: A priority-ordered pull, offered beside the required one
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
`dinah next` orders a state's ready cards by arrival, ties broken by creation ordinal, and consults priority nowhere. That is CORE-QUEUE-3 and it is correct: the operator ruled on 2026-08-23 that the profile governs, and dinah-197 corrects the design document sentence that claimed otherwise.

What that ruling deliberately left open is the thing people actually want. The board that motivated this whole slice, the AOY concept board at GK Software, leads its status document with a severity and priority table for every state, and somebody maintaining it wants the most important card next rather than the oldest one. Making levels visible was what forced the question; answering it is this card.

CORE-QUEUE-4 permits exactly this: "A tool MAY offer orders beside the one CORE-QUEUE-3 defines, provided the order CORE-QUEUE-3 defines remains available." So a priority-ordered pull is legal without amending any conformance MUST, provided the arrival order stays reachable and stays the default.

What needs deciding rather than falling out of the code: how a caller asks for the other order, whether a workbench may declare which order it prefers, and where unprioritised cards sit. The design document's rejected sentence already proposed an answer to the last one, "cards without a priority after all cards with one", which is worth reusing rather than re-deriving. The order is the declared order of the workbench's own priority set, which the level model already carries as a rank per axis, so nothing new is needed to know which priority outranks which.

Out of scope: changing what CORE-QUEUE-3 requires, and changing what `dinah next` does by default.
