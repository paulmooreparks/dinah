---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:56Z
ordinal: 19
note: "Show's composed branch returns text today and runShow writes it whatever the format, so `dinah show pb-1/comments/1 --json` prints frontmatter rather than JSON. Folding the collection listing into that same text stream would repeat the defect on a surface being built now, and it would also leave the empty collection printing nothing at all, which is the confident-empty-answer family this card belongs to. So the collection form gets a listing, a JSON shape and an empty sentence. The asymmetry this leaves, a collection answering JSON where its own member answers text, is real and is named in the spec's out-of-scope list: no card carries that defect today, and fixing it changes a shipped surface for every composed reference rather than adding a new one."
---
Library.Show gains a return value carrying a CollectionListing, so the collection form answers a shape under --json, and the member form's existing raw-text behaviour under --json is left exactly as it is.