---
kind: decision
state: resolved
ts: 2026-09-14T02:16:31Z
ordinal: 12
note: These three are where the measured traffic is (claim 10x, ls 3.9x, next 2.5x the human form, bytes as proxy). Response is a single shared struct reached from two call sites (emit, emitWorkstream) rather than 22 separate shapes, so covering it costs one encoding, not 22. Everything else (Status, Tree, Detail, CheckReport, Settings, Identity, etc.) is a lower-frequency read in a driver loop; adding compact renderings for those is separate, later design work the fallback rule does not block.
---
The compact projection covers exactly three shapes: *verb.Response (so claim, move, release, block, unblock, add, card set, comment, attach, archive, delete, rename, pull, join, leave, workbench set, and the two workstream acts, workstream new and workstream set, all get it through one shared encoding), Listing (ls), and []Offer (next). Every other shape emitJSON handles today keeps emitting canonical JSON when compact is requested.