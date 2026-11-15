# Independent chat cleanup acceptance checks

The checks are frozen from docs/design/chat-cleanup.md at 47cbcb68e920b667d1075e3b6669ac2ef65651c1 and the trusted assignment in bus message 8489. No implementation was read to write them. The chain and loaded-without-unload requirements come from that assignment; their contract amendment was still pending at freeze time.

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

The source was formatted with gofmt. The FIFO compile-only command `go test -tags quality_chat_cleanup ./cmd/dinah-desktop -run '^$'` passed with no tests selected (0.204 seconds). This verifies compilation only. No product acceptance case has run, and no provider or personal home has been accessed. The implementation handover must supply the clock, provider stand-ins and durable observations before these checks can be bound.
