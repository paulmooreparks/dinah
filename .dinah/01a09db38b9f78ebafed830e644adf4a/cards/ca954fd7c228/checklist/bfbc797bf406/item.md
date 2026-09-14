---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:24Z
ordinal: 12
note: "citation: scheme=command, target=git diff --name-only origin/main...HEAD on dinah-406-nothing-holds-the-extension-s-message-catalogue-to-the-code-that-reads-it-in-either-direction at dbe5103, observed: four paths and no fifth. They are editors/vscode/test/fixtures/unresolvable-key-call-site.ts.txt, editors/vscode/test/unit/l10n-keys.test.ts, editors/vscode/test/unit/l10n-placeholders.test.ts and internal/msg/msg_test.go, which is exactly the spec's Files table. Nothing under editors/vscode/src/ and no production module, so the packaged extension is byte-identical and .vscodeignore needs no new entry."
---
The diff adds no file under `editors/vscode/src/` and changes no production module. Verified by `git diff --name-only origin/main...HEAD`, whose output is exactly the four paths the spec's Files table names. A fifth path in that output is a finding to explain rather than a step to wave through, because the packaged extension is meant to come out byte-identical and `.vscodeignore` is meant to need no new entry.