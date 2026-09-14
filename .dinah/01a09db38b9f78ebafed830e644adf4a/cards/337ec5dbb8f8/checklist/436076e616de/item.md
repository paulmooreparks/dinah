---
kind: decision
state: resolved
ts: 2026-09-14T02:18:04Z
ordinal: 19
note: A built dinah.exe sitting in a developer's checkout embeds the guide's bytes and therefore contains the name, and it is git-ignored rather than absent. A walk over every file would fail on it and the failure would be nobody's defect. The extensions read are .go, .md, .ts, .mjs, .py, .txt, .json, .ndjson, .yml, .sh, .ps1, .mod and .sum, which covers every source file in the tree at b825059, and the walk skips dot-directories and node_modules, dist, out, bin and __pycache__.
---
The DinahPath name guard reads a declared set of text extensions rather than every file in the tree.