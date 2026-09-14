---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:17:53Z
ordinal: 36
note: "English moves from \"Slug\" to \"Reference\", because the cell comes to hold a reference. German moves from \"Slug\" to \"Referenz\" and Hindi from \"उपनाम\" to \"संदर्भ\", both taken from `column.tree.reference`, which is the other standalone listing carrying an address and already renders the same English word that way. Read against the current English and its context, neither old rendering survives: \"उपनाम\" is a nickname or handle and no longer describes the cell, and the German entry carried verbatim true on the claim that \"Slug\" is the German word too, which cannot be said of \"Referenz\", so the flag goes. The source both entries carry is msg.Fingerprint(\"Reference\") = fe88d270aa3e71da, which is what `column.tree.reference` already stores in both catalogues. The five skeleton catalogues carry \"Reference\" with skeleton true and no source. This is a mechanical substitution taken from a sibling entry rather than a fresh rendering of a clause, and no fluent reader has seen it."
---
`column.workstreams.reference`: replaces `column.workstreams.slug`; retranslated in German and Hindi because the English word changed and the German entry's verbatim claim stops being true.