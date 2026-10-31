# Renderer message identity

Front End owns the renderer correction from released 40d133a, separately from accepted resume/UI and toolbar candidates. Engine owns adapter and saved capture. The additive contract is recorded in Engine diagnosis f6a92810 and bus 2468/2470. Original evidence and the captured unfinished item's fate remain Engine-owned; no intended sentence or upstream cancellation is inferred.

## Source finding and representation

The actual appendChunk function previously keyed its current accumulator only by text category. A source-level harness reproduces the captured shape: a notice clears the current pointer, a late old-item suffix creates another bubble, and a distinct new assistant item appends to that bubble. The initial reproduction uses DOM-row stubs and is retained separately from routed browser verification.

Identified updates require nonempty turnId/itemId strings and append or snapshot contentMode. The renderer keys conversation, category, original turn and item. Notices can reset the legacy current pointer without removing identified attribution. Reasoning and assistant output remain separate. A snapshot replaces and finalizes only its matching item, including empty text. Repeated snapshots produce no extra row; a later append to a completed item is ignored. A later authoritative snapshot can replace the same item again. Empty identified text hides its empty bubble until authoritative nonempty replacement arrives.

Missing identity or valid mode follows the legacy append path, which never appends to an identified accumulator. Existing saved rows are restored at their recorded boundaries without rewriting files or guessing lost identities. New identified saved rows restore their optional identity and completion fields; a partial saved row remains open to checkpoint catch-up, while a completed row is finalized.

## Ledger lifecycle

Each log object owns an identity ledger through a WeakMap. This scopes attribution to retained conversation/log state rather than a process-global tombstone set or one active item. A trimmed row leaves a retired key with no DOM row reference, and a later append or snapshot for that key is ignored. A disconnected row is also retired before reuse. This prevents a late update from reviving removed display history.

Log rebuilds explicitly discard the old log's index. New saved display copies and replay logs receive their own indexes; returning to a kept log preserves that log's attribution. Existing journal-generation guards reject obsolete-generation events before renderer updates. A newly replayed current generation can rebuild rows for its original identities without retaining references to replaced DOM. The ledger contains identities admitted during a retained log epoch, including retired keys; it has no arbitrary eviction that would permit an old key to become new again. Replacing or releasing a log releases its ledger. This is a lifecycle bound, not a claim of a fixed number of keys independent of epoch length.

## Verification boundary

The actual-function harness covers item separation, completion, authoritative empty text, legacy isolation, retirement and conversation scope. Routed browser coverage is prepared for live/batched replay/saved equivalence, notices, same-turn and cross-turn identities, reasoning, partial saved catch-up, full replay reset, obsolete generations, close/reopen and trim retirement. Exact commands, outcomes and any failed harness attempts remain in .diagnostics/message-boundaries-20261101 and will be recorded before handoff. Engine independently verifies reconstructed-fragment and complete replacement redaction before persistence.

No original transcript is migrated, rewritten or reopened. No synthetic test establishes what happened to the earlier unfinished native item. Quality retains independent review and Release retains merged gating; no running app restart is authorized here.
