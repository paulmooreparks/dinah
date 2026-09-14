---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:36Z
ordinal: 13
note: "catalog: internal/msg/locales/en.json, refusal.unresolved-item.next, now \"; answer it, then resolve, verify or fail that item\", with the context rewritten so it no longer says the edit is made in the file because nothing in Dinah writes a checklist item. The same correction landed in the other seven files in the same commit: de and hi carry real translations with a recomputed source fingerprint, and af, cs, es, fil and id carry the English under skeleton: true, which is the discipline their catalogs already follow. Visible in the run above: the trunk binary answered `; answer it, then mark that item resolved in checklist/25196d87ff73/item.md` where the new binary answers `; answer it, then resolve, verify or fail that item` on the same workbench and the same refusal. `go test ./internal/msg/...` passes, which includes TestATranslationTracksItsEnglishSource (the guard that would fail on a stale fingerprint) and TestASkeletonEntryReallyCarriesTheEnglishText."
---
`internal/msg/locales/en.json`'s `refusal.unresolved-item.next` no longer asserts that nothing in Dinah writes a checklist item; its corrected text names the actual verbs (`resolve`/`verify`/`fail`), and the same correction is made in the other seven locale files' equivalent entries in the same diff.