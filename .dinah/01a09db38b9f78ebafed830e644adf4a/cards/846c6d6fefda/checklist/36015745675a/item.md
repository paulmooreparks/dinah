---
kind: decision
state: resolved
column: c9428b3bc921
ts: 2026-09-14T02:17:11Z
ordinal: 11
note: The discovery override (--workbench/DINAH_WORKBENCH pointed at an exact anchor directory) and the native-home boundary rung both depend on benchIn's unconditional bare-anchor check, per bench.go's own comments; converting every fixture entry to the container shape would silently drop the only coverage of that path.
---
The bare-workbench-directly-at-a-directory layout (no .dinah anywhere on its path) stays covered in the reworked fixture via `sibling`, deliberately unchanged and labeled as a layout no command produces. atRoot and nested move to the real .dinah-container shape dinah init actually writes.