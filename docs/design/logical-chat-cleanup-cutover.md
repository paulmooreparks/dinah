# Logical-chat resolver replacement

Engine proposes this integration plan to Release on 15 November 2026. Cleanup has already shipped with the interim resolver. Release owns the durable model and its resolver bodies; Engine owns cleanup lifecycle, archives and the organisation-chart capture/query. This note requires Release's agreement before the replacement is integrated.

## Consumer contract

The replacement preserves LogicalChatMembers and the pure logicalChatMembersFromSettings signature. The inventory and metadata signatures in logical-chat-query-seams.md also remain unchanged. The chart's App call, pageMethods entry and frontend DTO do not change.

Conversion preserves each established ChatID and its aliases, current head and oldest-first owned member order. Known abandoned candidates remain owned members. Unknown allocation outcomes retain their attempt identity without a fabricated native ID. Forks and workspace moves remain separate recipients. Ambiguity and live allocation return visible refusals, including when a caller only has a physical session reference.

The model may represent additional logical-only and repair records through the agreed inventory and metadata queries. That extension must not admit provider-history-only records or revive finalized tombstones.

## Reservations and stored archives

Engine continues writing existing RemovedChats reservations through updateSettings. Release's shared transaction hook captures those reservations in the model generation and admission checks. Any owned reservation blocks ordinary admission, including an unknown future phase. Cleanup re-resolves and compares Generation under the same updateSettings transaction before reserving a mutation.

The conversion preserves every existing Group, member snapshot and lifecycle phase. A retained archive's frozen member snapshot remains the authority for restoration and final deletion. The replacement must not recalculate that archive from newer ownership or erase a reservation while reconciling its projection. Interrupted retaining, restoring and finalizing operations continue through the existing recovery paths.

The model and legacy projection use Release's recoverable sidecar transaction and baseline reconciliation. An older build's settings write must be reconciled from the recorded baseline before admission resumes. A conversion failure refuses affected operations visibly and leaves the retained archive recoverable. Integration must follow the accepted logical-chats.md recovery design rather than introduce a second conversion protocol here.

## Verification before integration

The candidate first runs the same invented valid ownership fixtures through the interim and durable resolvers. Comparisons cover stable identity, head, ordered physical members, separate recipients and refusal behavior. Generation values may differ between implementations; each implementation must detect a reservation or ownership change between query and mutation.

The frozen cleanup checks then run unchanged on the combined candidate. They cover staging, Restore, final deletion, expiry, loaded providers, pending allocations and restart recovery. The linked document records, drawing authority and composer-state checks remain required. Tests seed retained archives before conversion as well as chats converted before entering the bin, so both directions exercise the same lifecycle contract.

The Codex descendant guard continues using the captured pure member query and refuses uncertain ownership without a native delete. Its supported-delete and independent-recipient controls run again after replacement. The chart runs its canonical identity, repair, reservation and provider-start checks against the new bodies, followed by the hidden real-query page check.

Release hands over an immutable wired candidate with its conversion and recovery evidence. Engine combines it on an isolated branch and records the unchanged cleanup acceptance results. Team Lead decides integration after that evidence is complete. No model stage is imported into the standalone chart merely to make this plan executable.
