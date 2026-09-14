---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:11Z
ordinal: 10
note: "This is not a new question: soleBench already carries the ambiguous list and walkFor already folds a child's ambiguous candidates into Enumerate's result with no dedicated bucket. Giving the root a special-cased report field would duplicate the same shape TreeVocabularyReport already handles for two ordinary sibling workbenches."
---
A root whose own .dinah holds more than one workbench is reported the same way an ambiguous descendant .dinah already is: every candidate listed as its own Candidate, none silently chosen, no new field added to any report shape.