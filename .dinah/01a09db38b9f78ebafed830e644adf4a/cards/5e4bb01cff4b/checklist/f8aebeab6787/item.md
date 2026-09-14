---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:01Z
ordinal: 13
note: "Test-stage re-verification. get_column(column_id=\"4b38abe7ebd5\", fields=\"instructions\") carries the \"## A changed or new English key re-runs the glossary sweep\" subsection immediately after AC-11's. Note: its live text is NOT byte-identical to the spec any more, and correctly so: the orchestrator corrected the false TestATranslationTracksItsEnglishSource claim per OQ-5's operator ruling, replacing it with \"No test in the tree produces that grouping. The method is the short script recorded in dinah-252's spec...\". This is the right live state, not a regression."
---
Agent Code Review's column instructions (id 4b38abe7ebd5) carry a second new "## A changed or new English key re-runs the glossary sweep" subsection, verbatim as given in the spec's Layer 1 "mechanism for adding a term later" section, naming the review-time check a diff adding or changing an en.json key must pass and citing this card's four seeded terms as precedent.