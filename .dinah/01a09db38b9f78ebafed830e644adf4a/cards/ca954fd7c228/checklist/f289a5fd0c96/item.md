---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:23Z
ordinal: 7
note: "citation: scheme=test, target=editors/vscode/test/unit/l10n-placeholders.test.ts#\"de keeps the placeholder names its English carries, and invents none\", observed before=fail after=pass. The assertion is the deepEqual over verdict.invented. Performed: de.json's dialog.card.copiedRef changed to \"{ref} kopiert nach {ziel}\", and the run reported exactly `de/dialog.card.copiedRef: {ziel}`. The same three sibling guards stayed green through that plant (exit 0, 0 failures). Comparison is by set: placeholdersIn returns a Set, so a name the translation carries fewer times than the English does is not reported, which the fixture test pins with a key whose two names are reordered. Restored byte-identically to green."
---
The same file reports every placeholder name a translation carries that its English does not, which is the direction neither the extension nor `internal/msg` catches today. Armed by changing `de.json`'s `dialog.card.copiedRef` to `{ref} kopiert nach {ziel}`, a string that renders the literal characters `{ziel}` at a reader because nobody passes that name. Against the unmodified tree it fires zero times over 826 key pairs. Comparison is by set rather than by count, so a translation naming a placeholder fewer times than the English does is not reported.