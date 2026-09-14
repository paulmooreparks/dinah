---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:11Z
ordinal: 31
note: "`docs/design/format.md:826` says \"A name carrying no dot is one of the twenty-three, or one a different build wrote\", and no set in that section has twenty-three members at `9260a2a`: declared thirty-two, written thirty-one, queryable twenty-nine, overlap twenty-eight, card-journal twenty-seven. The sentence is about any core event name a reader may meet, so the set it means is the declared one, which reads thirty-five once this card's three constants land. The figure is not this card's defect, and it sits in the one section this card moves every count around, so correcting it while the section is open costs one word and leaving it costs the next reader a contradiction between a corrected paragraph and an uncorrected sentence three paragraphs down. Correcting alone would not be enough: the reason it has been wrong is that `derive_event_counts.py` checks five counts and three placements and never reads that sentence, so the script gains the sixth claim in `claims()` at `scripts/derive_event_counts.py:139`. This is the workbench's own worked example applied to itself, computing a figure where it is asserted rather than carrying it forward from prose."
---
The format document's stale "twenty-three" figure is corrected on this card rather than filed, and `derive_event_counts.py` gains a sixth claim so it cannot rot again.