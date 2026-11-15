# Cleanup host binding progress

Quality integrated candidate 61dd1e66 into quality/chat-cleanup without changing product source. The frozen assertions remain unchanged. The new test-only adapter binds the draft and expiry-boundary cases to RemoveChats, RestoreRemovedChat, DeleteRemovedChat, SaveUIState and the actual retention sweep. Restart creates a new App on the isolated home; it does not run the full background startup sequence.

Draft values are read directly from persisted ui-state.json. The provider-delete hook checks that the retained manifest still exists before native deletion and records the invented provider's actual outcome. The adapter reads live or archived transcript bytes and retained settings. It fails explicitly for unbound runtime, bus-routing, linked-record and lease observations. It also refuses to synthesize the expiry result required by the provider-outcome test from expected state.

The initial selected run used FIFO ticket 2176 after the priority document investigation. Draft preservation and stale-save cases for final deletion and expiry, plus the exact thirty-day boundary, passed in 1.954 seconds. The remaining startup counter recipe was requested from Engine in message 8764. Release's durable model, combined drawing authority and rendered/asset checks remain outside this partial host binding.

Provider deletion cases then passed in 10.131 seconds for supported, unsupported and failing deletion through Delete Now and Empty Bin. The frozen checks observed the retained archive at every supported/failed native callback, verified local erasure and tombstones, checked providerCopyRemains and its reason, and reopened the isolated home without resurrection. Provider state is an invented callback-owned copy; no native account or service was contacted. The expiry provider-result matrix remains unbound because the sweep has no returned result, although the separate expiry-boundary and stale-save cases ran.

Commands:

- `go test -tags quality_chat_cleanup,chatcleanup_acceptance ./cmd/dinah-desktop -run 'TestQualityCleanupDraftsAndStaleSave|TestQualityCleanupExpiryBoundary' -count=1 -timeout 60s`
- `go test -tags quality_chat_cleanup,chatcleanup_acceptance ./cmd/dinah-desktop -run '^TestQualityCleanupProviderDeletion/(supported|unsupported|failing)/(delete|empty)$' -count=1 -timeout 60s`

Both commands ran through scripts/with-test-lock.ps1. Engine confirmed the startup observation gap in message 8770 and is supplying counted launch adapters with a live control. Those frozen cases remain unbound. These results accept only the exercised host behaviors, not the combined cleanup product.

## Counted chat startup and bus dormancy

Quality applied Engine's test-only addition dfe4a3fd49316a70d809221e26c18f2875dd361f over combined candidate 61dd1e66, producing local cherry-pick ceac9198. The binding now replaces every provider adapter before constructing App. It counts actual launches, load/resume and prompt RPCs, and requests to open chat windows. Restart reconstructs App on the same isolated home and runs reconciliation, queue restoration, ChatWindows and real bus discovery/delivery. Restore also runs a discovery round. This does not exercise unrelated App.startup maintenance or a native desktop restart.

The unchanged frozen dormancy assertions pass. Two retained deliveries, transcript bytes, saved title and bus identity survive binning and Restore. Direct and addressed group sends refuse the removed recipient; a group broadcast adds no delivery. Binning, message attempts, restart discovery and Restore produce no launches, loads, prompts or window-open requests. The tagged positive control passes and observes an explicit reopen launching and loading once, followed by one prompt through the real turn path. The ordinary cleanup listing, Decisions query and bus availability are read from their real APIs. The fixture creates no pending Decision, so this run does not prove preservation or filtering of existing Decision requests.

FIFO ticket 2191 was withdrawn before execution to finish the query binding. Ticket 2194 failed the group-refusal assertion because Quality supplied the group name without its required # prefix. That was a binding defect before group routing, not a product refusal result. Quality corrected only the address in the binding. The frozen assertions were unchanged. The corrected run passed in 5.319 seconds with exit zero, including the positive control, dormancy, draft/stale-save, exact expiry-boundary and six supported/unsupported/failing Delete Now and Empty Bin cases.

The command ran through scripts/with-test-lock.ps1:

`go test -tags quality_chat_cleanup,chatcleanup_acceptance ./cmd/dinah-desktop -run '^(TestQualityCleanupDormancy|TestCleanupAcceptanceRestartCountsRealProviderEntry|TestQualityCleanupDraftsAndStaleSave|TestQualityCleanupExpiryBoundary)$|^TestQualityCleanupProviderDeletion/(supported|unsupported|failing)/(delete|empty)$' -count=1 -timeout 90s`

Runtime working/pinned and loaded-without-unload cases, switched-chain ownership through Release's durable model, linked document/draft records, combined drawing authority and the expiry provider-result matrix remain unbound here. Front End's separate combined rendering evidence is d474c412; this run does not repeat it. Engine's newer integration 9ed1bc16 has not been tested by this binding. No real home, provider process, account, remote service, visible window or install was used. Quality retains the remaining independent checks for the integrated handover.
