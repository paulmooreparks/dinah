---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:23Z
ordinal: 1
note: "citation: scheme=test, target=editors/vscode/test/unit/l10n-keys.test.ts#\"every message name the extension asks for is one the catalogue carries\", observed before=fail after=pass. The assertion is the deepEqual over sweep.literals filtered against en.json's key set. Performed: t(\"tree.group.ready\" changed to t(\"tree.group.redy\" in src/tree.ts, tsc exited 0 so the plant type-checks and the failure came from the assertion, and the run reported `src\\tree.ts:526 tree.group.redy` (file, line and key), the spec's own observed text. Restored from a byte-identical copy and the file went green again. Against the clean tree the check fires zero times over 101 literal keys (97 distinct) and 118 catalogue keys. Reviewer's command: cd editors/vscode && npm run test:unit."
---
`editors/vscode/test/unit/l10n-keys.test.ts` fails when a localizer call site names a key `src/locales/en.json` does not carry, and passes against the tree as it stands. Armed by changing `t("tree.group.ready"` to `t("tree.group.redy"` in `src/tree.ts`; the plant type-checks, so `npm run compile-tests` succeeds and the failure comes from the assertion rather than from a build error. The failure names the file, the line and the key. Restoring the literal from a byte-identical copy returns the file to green, and that green run is the clean case: without it the criterion would pass against a sweep that reported every key.