---
kind: decision
state: resolved
ts: 2026-09-14T02:17:02Z
ordinal: 24
note: "Design review found the first draft's D-9 sound on cost/determinism grounds but its enforcement path unstated: Agent Code Review's live column instructions were read directly and carry no rule resembling \"check for a locale-diff decision record\" and no reference to workbench document 51. dinah-259, which this card already cites as evidence, is exactly what slips through a guard that exists only as an expectation: fluent, current, on-glossary, and wrong in meaning. The fix adds a subsection to that column's instructions (new AC) so the next reviewing agent inherits the check the same way it already inherits the corpus and prose-standard checks, rather than depending on having read this card's spec."
---
The semantic layer's enforcement is wired into Agent Code Review's own column instructions, not left as a procedural expectation nothing structural carries.