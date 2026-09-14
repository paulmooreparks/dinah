---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:23Z
ordinal: 4
note: "citation: scheme=test, target=editors/vscode/test/unit/l10n-keys.test.ts#\"every catalogue key is one the extension actually reaches\", observed before=fail after=pass. The assertion is the deepEqual over en.json's keys filtered against the reached set (resolved literal asks plus family members prefixed). Armed by adding \"history.event.bogus\" to the eight runtime catalogues, which this check reported as `history.event.bogus`. Against the clean tree all 118 keys are reached and it fires zero times. l10n.test.ts's \"every held-card key the catalogue declares is one the rendering reaches\" is untouched and still ships."
---
Every key in `src/locales/en.json` is either a resolved literal ask or a member of a declared family, and a key that is neither fails the test naming it. Armed by adding an entry to `en.json` that nothing renders. The clean case is the unmodified tree, where 118 keys are all reached and the check fires zero times; the existing `"every held-card key the catalogue declares is one the rendering reaches"` in `l10n.test.ts` stays as it is and is not replaced by this.