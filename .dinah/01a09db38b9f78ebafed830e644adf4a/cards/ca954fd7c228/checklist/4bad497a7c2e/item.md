---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:24Z
ordinal: 10
note: "citation: scheme=test, target=editors/vscode/test/unit/l10n-placeholders.test.ts#\"a manifest value carries no placeholder, because VS Code fills none\", observed before=fail after=pass. It walks all eight package.nls*.json files and reports tag, key and value for any value containing { or }. Armed by adding {x} to package.nls.json's manifest.viewsContainers.dinah.title, which reported `en/manifest.viewsContainers.dinah.title: Dinah {x}`. The values counter carries its own floor, armed separately by making the walk read no file at all, which gave \"no manifest value was read at all, so this guard is asserting nothing\". Measured against the clean tree: 312 values across eight files, zero carrying a brace."
---
A test asserts that no value in any of the eight `package.nls*.json` files contains a `{` or a `}`, and reports the tag, key and value of one that does. Armed by adding `{x}` to any value in `package.nls.json`. Against the unmodified tree it scans 312 values and fires zero times, and the count of values scanned carries its own `> 0` floor.