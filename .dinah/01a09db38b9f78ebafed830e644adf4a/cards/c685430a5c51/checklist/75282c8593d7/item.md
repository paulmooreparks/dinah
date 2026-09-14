---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:06Z
ordinal: 12
note: "This completes the parent rather than disputing it. The parent's operative rule is that attach refuses when `bench.MountOf(kind, bench.AttachmentsDir)` reports no mount, and that rule already covers the workstream: D-5 keeps a workstream out of the containment table, so MountOf answers false for it. Only the parenthetical \"which is item and attachment\" is short. Verified at 22a35fc by running `dinah attach workstream/probe-stream f.txt`, which exits 0, prints nothing at all because the response carries no card, writes `workstreams/ab5329af915d/attachments/6d7fab9c4d8b/payload/f.txt`, and leaves `dinah path workstream/probe-stream/attachments/1` refusing dinah.unknown-workstream and `dinah contents workstream/probe-stream` reporting nothing."
---
The refusal fires on three kinds, not the two dinah-456 section 5.6 enumerates, because the workstream is reachable and mounts nothing.