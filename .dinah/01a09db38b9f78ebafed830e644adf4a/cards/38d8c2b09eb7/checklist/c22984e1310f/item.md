---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:38Z
ordinal: 5
note: Finding 3. The verdict is the packaging script's own exit status against a real archive rather than a reading of the README, and the three-way comparison is what catches a correction that fixes the sentence into a different falsehood.
---
`npm --prefix editors/vscode run package` followed by `node editors/vscode/scripts/verify-package.mjs` exits 0, and the README's pre-release paragraph read beside that script's PRE_RELEASE_PROPERTY assertion and beside the publish step at .github/workflows/vscode-release.yml:272 says the same thing all three do, which is that no published archive carries the mark.