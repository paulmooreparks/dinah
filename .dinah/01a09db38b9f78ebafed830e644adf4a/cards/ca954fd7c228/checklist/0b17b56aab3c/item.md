---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:23Z
ordinal: 6
note: "citation: scheme=test, target=editors/vscode/test/unit/l10n-placeholders.test.ts#\"de keeps the placeholder names its English carries, and invents none\", observed before=fail after=pass. The assertion is the deepEqual over verdict.dropped. Performed: de.json's dialog.card.copiedRef changed from \"{ref} kopiert\" to \" kopiert\", and the run reported exactly `de/dialog.card.copiedRef: {ref}`, the spec's own observed text. With that same plant in place I ran l10n.test.js, l10n-staleness.test.js and l10n-coverage.test.js, which carry the parity, honesty, glossary and staleness guards, and all three exited 0 with 0 failures, so the spec's central claim is now verified rather than asserted. Restored byte-identically to green. Against the clean tree, zero fires over 826 pairs."
---
`editors/vscode/test/unit/l10n-placeholders.test.ts` reports every placeholder name an entry's English carries that the same key's translation does not, one test per non-English tag, each failure reading `tag/key: {name}`. Armed by changing `de.json`'s `dialog.card.copiedRef` from `{ref} kopiert` to ` kopiert`, which leaves the parity, honesty, glossary and staleness guards green and turns this one red. Against the unmodified tree it fires zero times over 826 key pairs.