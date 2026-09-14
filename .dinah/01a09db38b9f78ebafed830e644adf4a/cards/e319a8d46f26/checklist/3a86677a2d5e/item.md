---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:52Z
ordinal: 59
note: "`copyCardRef` and `copyWorkbenchPath` are `fanOut` with effect `oneCall`: one clipboard write over the whole selection. Round 5 left how that happens to the implementer, and AC-19's arm pinned \"one host call carrying both rows' values newline-joined\", which fills `dialog.card.copiedRef` (\"Copied {ref}\") with a newline-joined list.\n\nTwo alternatives were rejected. Having the first row's act do the write and the rest do nothing is a loop that behaves differently on one iteration, which is the kind of special case §3a exists to remove. Having `invoke` do the write before calling `runBulk` puts an effect and a message outside the perimeter, which is round 2's blocker returning.\n\nWhat ships: `BulkDeps` carries `finish`, the one act performed after the loop over the rows that finished, which shows nothing at all, and `successMessage`, which composes the sentence a wholly successful run shows instead of `dialog.bulk.allSucceeded`. `summaryFor` takes it as case 5, ahead of the single-row case, so a one-row copy keeps today's singular toast and a two-row copy gets `dialog.card.copiedRef.many`. The command speaks only through the summary, which keeps route 1 the only route it has.\n\nBoth fields are optional and are declared by these two commands alone. Nothing static holds that to two, and AC-19's `oneCall` arm is what drives it behaviourally: an entry declared `oneCall` that reports per row fails the one-message clause. Plant H in the reductions is that shape and it reddens."
---
The two `oneCall` commands declare `finish` and `successMessage` on their deps and report nothing per row, rather than calling the host once inside the loop.