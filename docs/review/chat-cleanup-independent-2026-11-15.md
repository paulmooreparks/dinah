# Independent chat cleanup acceptance checks

The checks are frozen from docs/design/chat-cleanup.md at 47cbcb68e920b667d1075e3b6669ac2ef65651c1 and the trusted assignment in bus message 8489. No implementation was read to write them. The first freeze was 2e0f554f. Before handover, contract amendment d5a140a10f259a0a9f103e030361defea31a65d3 arrived and was read without inspecting implementation. This second freeze adds a history-only allocated target, mixed deletion support within the chain and protection by a working or pinned history member.

## Coverage

The tagged Go suite checks removal dormancy, direct and group refusal, broadcast exclusion, retained deliveries, restart exclusion and closed restoration. It exercises supported, unsupported and failing provider deletion through Delete Now, Empty Bin and expiry, observes provider deletion before local erasure, and checks providerCopyRemains and its reason. It also covers switched chains, loaded chats without unload, working and pinned refusals, mixed batches, idempotent removal and the thirty-day boundary.

The loaded-without-unload case requires the words "still loaded" in the authoritative outcome presented to the person. Its provider may remain resident until shutdown, but the chat must disappear from active surfaces immediately and must not load on restart.

## Binding requirements

The fixture is intentionally unbound. Running a case without a binding fails; it cannot report acceptance or silently skip. A later test-only binding must call the real host APIs and bus delivery paths, reopen the real isolated stores on restart and observe durable state. It must never implement an alternate cleanup state machine.

Seed creates invented nonempty transcript and settings data, a stable bus identity, a provider-owned copy and the requested pending deliveries. Chain identifies the saved predecessor and successor identities of one switched chat. Active means an identity is not binned; it does not require a separate visible row for every predecessor.

Starts, Loads and Wakes are cumulative observations from provider stand-ins that survive fixture restart. Restart closes the old host and creates a new host on the same isolated home. Loaded describes provider residency. Events record real provider deletion attempts, including failed attempts, and actual local erasure in order. Unsupported deletion must never invoke an invented delete command.

Act maps remove, restore, delete and empty to the documented host calls. Expire drives the actual retention sweep using a test-only clock. If the sweep has no public result, its result projection must use its actual finalization outcome, including the provider-copy reason. Advance changes that clock without sleeping. Observe reads stores and host listings; it must not infer results from the requested action.

Send exercises direct delivery, a group message explicitly addressed to the removed identity, and broadcast selection. Direct and addressed group delivery must refuse with a removed reason. Broadcast may simply exclude the identity. Every path must leave its queue and wake counters unchanged.

## Frontend receiving checks

The combined handover must also demonstrate that removal closes an open idle chat, removes its cached rows and notices, and retains each batch outcome in place. Restore must update listing metadata without opening a chat. The removed section must show the remaining retention and provider-copy warning. Event-before-result and result-before-event order must produce the same visible result without duplicate rows. These rendered checks remain pending until Front End hands over its candidate.

## Verification status

The source was formatted with gofmt. The FIFO compile-only command `go test -tags quality_chat_cleanup ./cmd/dinah-desktop -run '^$'` passed with no tests selected (0.204 seconds at the first freeze, then 0.211 seconds after the amendment). This verifies compilation only. No product acceptance case has run, and no provider or personal home has been accessed. The implementation handover must supply the clock, provider stand-ins and durable observations before these checks can be bound.

## Amended binding contract

The candidate supplies chatCleanupAcceptanceInstall, chatCleanupAcceptanceSweep and chatCleanupAcceptanceObserve under chatcleanup_acceptance. The adapter will use its Now, DeleteProvider and UnloadProvider hooks, plus independent counters and direct isolated-store inspection. HistoryOnly seeds an allocated switch target recorded only in transfer history. It must remain part of cleanup while not becoming the displayed current session. The test tag quality_chat_cleanup is combined with chatcleanup_acceptance at handover.

The native deletion event sequence must follow the oldest-first chain order before any local chain erasure. Per-member programCopies must describe every native result and the aggregate providerCopyRemains must be true when any member remains. These observations are additional receiving assertions to bind with the amended result shape; the factory must preserve their actual values.

## Composer draft amendment

Contract 6b756c8587d5d46ea1e348da35602611766c2442 adds durable composer drafts. The independent cases seed distinct drafts for the current and earlier switched sessions, preserve them through staging, restart and Restore, then check erasure through final deletion and expiry. Each case submits the original stale full-state payload after finalization and again after restart. Neither write may restore either draft, while an unrelated preference must survive.

SaveUIState must call the real host full-state save path with a copy of the supplied map. ReadUIState must read the persisted isolated ui-state.json, without filtering in the test adapter. The binder must not mutate the stale input map. The implementation review must also verify that tombstone filtering and finalization share serialization; sequential stale writes alone cannot prove their concurrency discipline. Frontend receiving checks must observe zero Sessions or SessionsAt calls during removal and Restore.

The amended suite passed FIFO compile-only verification in 0.210 seconds, with no tests selected. Acceptance remains pending the implementation handover.

## Referenced records and revoked leases

The independent ownership case follows contract c858e112fbce957a418b2b46d67badb9a3db77ba and assignment 8540. It seeds a switched chat referencing a document and a document draft, captures their bytes and records, and compares them after binning, Restore, final deletion and restart. Its remote-write counter must remain zero throughout. Document drafts are distinct from composer drafts, which the preceding tests erase at final deletion.

SeedLinkedRecords creates invented records in the real isolated stores and chat references to them. LinkedRecords returns detached snapshots of the referenced bytes and records. It also includes any associated comments, suggestions, attachments, publication records, workspace files, invented credentials and tool registrations that the fixture seeds. It must read actual persisted values without normalizing away changes. RemoteWrites records all mutating requests at the fake service after setup; documented provider-session deletion is observed separately and is not a remote content write.

OpenWithLeases uses the ordinary opening path with stand-in providers and returns the actual issued authority tokens by kind, including drawing. UseLease attempts a harmless authorized request through the real service, with no remote write. The test proves validity before removal, refusal after removal and Restore, fresh authority on explicit reopen, and continued refusal of old authority. StartPendingLeaseWork creates a cancellable request held before its effect. PendingLeaseWork observes actual cancellation and releases any held completion to prove revoked work cannot finish with authority. These bindings must cover every issued lease kind and must never invent token validity in test code. Missing bindings fail acceptance.

The ownership amendment passed FIFO compile-only verification in 0.284 seconds, with no tests selected. Runtime acceptance remains pending.
