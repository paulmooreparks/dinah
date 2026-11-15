# Logical-chat resolver replacement

Engine and Release agreed this integration plan on 15 November 2026, including Release review e0baa797. Cleanup has already shipped with the interim resolver. Release owns the durable model and its resolver bodies; Engine owns cleanup lifecycle, archives and the organisation-chart capture/query. The agreement defines a future integration gate and does not accept the isolated model stages for coupling.

## Consumer contract

The replacement preserves LogicalChatMembers and the pure logicalChatMembersFromSettings signature. The inventory and metadata signatures in logical-chat-query-seams.md also remain unchanged. The chart's App call, pageMethods entry and frontend DTO do not change.

Conversion preserves each established ChatID and its aliases, current head and oldest-first owned member order. Known abandoned candidates remain owned members. Unknown allocation outcomes retain their attempt identity without a fabricated native ID. Forks and workspace moves remain separate recipients. Ambiguity and live allocation return visible refusals, including when a caller only has a physical session reference.

The model may represent additional logical-only and repair records through the agreed inventory and metadata queries. That extension must not admit provider-history-only records or revive finalized tombstones.

## Reservations and stored archives

Engine continues writing existing RemovedChats reservations through updateSettings. Release's shared transaction hook captures those reservations in the model generation and admission checks. Any owned reservation blocks ordinary admission, including an unknown future phase. Cleanup re-resolves and compares Generation under the same updateSettings transaction before reserving a mutation.

The conversion preserves every existing Group, member snapshot and lifecycle phase. A retained archive's frozen member snapshot remains the authority for restoration and final deletion. The replacement must not recalculate that archive from newer ownership or erase a reservation while reconciling its projection. Interrupted retaining, restoring and finalizing operations continue through the existing recovery paths.

Startup acquires single-window ownership and reconciles the sidecar transaction and intervening older-build writes before enabling model-dependent admission or cleanup recovery. Existing archives may supply an error or repair representation while conversion is pending, but that read grants no mutation authority. Engine retains the sole cleanup recovery loop.

When settings match a pending transaction's prior baseline, recovery completes only its intended projection entries and finalizes the sidecar. When they match the intended projection, recovery finalizes the sidecar without replaying file or provider operations. When neither matches, recovery preserves allocation receipts and reimports changed entries against the intended baseline, marking affected ownership for repair. Conversion performs no provider operation or whole-settings rollback.

Newer ownership cannot widen an archive's frozen deletion set. A valid retained snapshot remains usable for its established lifecycle operation even when live ownership needs repair. A damaged or ambiguous retained snapshot refuses that operation visibly. Projection conflicts preserve every reservation. A recoverable ownership conflict affects its chats only; an unreadable or untrusted sidecar visibly blocks dependent operations without falling back to the interim resolver. Integration follows logical-chats.md and leaves retained data available for repair.

## Verification before integration

The candidate first runs the same invented valid ownership fixtures through the interim and durable resolvers. Comparisons cover stable identity, head, ordered physical members, separate recipients and refusal behavior. Generation values may differ between implementations; each implementation must detect a reservation or ownership change between query and mutation.

The frozen cleanup checks then run unchanged on the combined candidate. They cover staging, Restore, final deletion, expiry, loaded providers, pending allocations and restart recovery. The linked document records, drawing authority and composer-state checks remain required. Tests seed retained archives before conversion as well as chats converted before entering the bin, so both directions exercise the same lifecycle contract.

Recovery fixtures interrupt retaining, restoring, finalizing, restored_cleanup and erasing boundaries and prove that only Engine's intended next operation completes. A future phase remains held. Another fixture changes one chat through an intervening older-build write while a retained group's values and archive remain byte-equivalent. Fault injection covers sidecar preparation, projection writes and sidecar finalization, including a foreign legacy write between recovery steps. Conversion provider-call counters must remain zero.

The Codex descendant guard continues using the captured pure member query and refuses uncertain ownership without a native delete. Its supported-delete and independent-recipient controls run again after replacement. The chart runs its canonical identity, repair, reservation and provider-start checks against the new bodies, followed by the hidden real-query page check.

Release hands over an immutable wired candidate with its conversion and recovery evidence. Engine combines it on an isolated branch and records the unchanged cleanup acceptance results. Team Lead decides integration after that evidence is complete. No model stage is imported into the standalone chart merely to make this plan executable.
