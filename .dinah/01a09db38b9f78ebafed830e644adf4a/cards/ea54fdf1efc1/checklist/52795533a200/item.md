---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:11Z
ordinal: 32
note: "The contract's first branch asks for one item per changed key and governs a diff changing existing translations; the second asks for one collective provenance item and governs a diff carrying a block of entries across a mechanical change. This diff changes seven existing translated keys in German and five in Hindi, which is the first branch and is recorded key by key in the items below, and it adds twenty-two new keys whose German and Hindi were written fresh, which no branch names outright and which is closest to the second, so it is recorded as one provenance item. Deleted keys carry no judgement about a translation and are recorded once, collectively. The two counts are derived rather than carried forward from this note's earlier reading of five: `refusal.dinah.unknown-path.next-addressed` and `refusal.dinah.not-renamable` changed in German only, because in each case the English was already free of the defect and only the German carried a word agreeing with an interpolated kind. Derivation: load `internal/msg/locales/{de,hi}.json` at 73f9590 and at the branch head and compare the `text` of every key present in both; German answers seven, Hindi five."
---
The locale record takes both branches of the translation staleness contract, because this diff does both things the two branches separate.