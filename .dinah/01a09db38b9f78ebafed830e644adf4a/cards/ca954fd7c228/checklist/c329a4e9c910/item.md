---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:23Z
ordinal: 8
note: "citation: scheme=test, target=editors/vscode/test/unit/l10n-placeholders.test.ts#per-tag floors, observed before=fail after=pass. Each of the seven per-tag tests asserts verdict.pairs > 0 and verdict.placeholders > 0 before it asserts the two finding lists. Armed twice: emptying de.json to {\"tag\":\"de\",\"entries\":{}} failed the de test alone on \"de shares no key with the base catalogue, so this guard compared nothing\" while the other six stayed green, which is the isolation the criterion asks for; and narrowing the placeholder pattern to match nothing failed every tag on the second floor, \"af read no English placeholder at all, so this guard is asserting nothing\", rather than riding the pairs count. Populations measured at this commit: de+hi 236 pairs and 244 English placeholders, af+cs+es+fil+id 590 and 610, all seven 826 and 854, all reproducing the spec exactly."
---
Each of the seven per-tag placeholder tests asserts its own `pairs > 0` and `placeholders > 0` before it asserts the two finding lists are empty, so the translated pair of languages cannot be hidden behind the five skeletons or the other way round. Armed per tag by emptying that tag's catalogue to `{"tag":"de","entries":{}}` and watching that tag's test fail on its own floor while the other six stay green.