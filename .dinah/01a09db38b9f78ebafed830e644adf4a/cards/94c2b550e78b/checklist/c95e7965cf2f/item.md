---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:33Z
ordinal: 21
note: "Splitting internal/guide/guides/references.md's \"Reading the archive\" section on blank lines gives five paragraphs, and each opens with one statement about how archiving or restoring behaves. Re-run at Agent Design Review against 866221f. Four phrases find nothing outside the guide: \"archive mirror at a reference\", \"Positions under the flag\", \"says so on its own first line\" and \"scans both halves\". Two do return hits, and neither reads the guide. \"restored column lands\" matches an error message at cmd/dinah/restore_test.go:460 that resembles the sentence, and \"travelled into the archive\" matches two error messages about a lock at internal/verb/beyond_test.go:1570 and :1795. The original note said the second of those returned nothing outside the guide, which is the one detail corrected here; the conclusion is unchanged, because an error message about a lock does not read the guide. The card's premise that nothing reads these five statements holds."
---
Five archive statements are pinned, not four. The card's count was one short.