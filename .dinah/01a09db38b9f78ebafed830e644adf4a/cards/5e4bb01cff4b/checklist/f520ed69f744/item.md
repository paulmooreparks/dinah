---
kind: decision
state: resolved
ts: 2026-09-14T02:17:01Z
ordinal: 17
note: "dinah-251's implementer measured the identical-English-implies-identical-translation candidate directly and found seven groups that still diverge after Hindi's own fix, at least two of them correctly (German gendered articles: \"Was sie tut\" vs \"Was er tut\"). A rule built on that invariant cannot ship. The term-trigger design checks presence of the right word wherever the concept appears, which survives legitimate surrounding-sentence variation."
---
The glossary is a term -> per-language accepted-forms declaration triggered by an English phrase, not a check that groups keys by identical English text and demands one rendering per group.