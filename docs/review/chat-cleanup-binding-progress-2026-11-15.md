# Cleanup host binding progress

Quality integrated candidate 61dd1e66 into quality/chat-cleanup without changing product source. The frozen assertions remain unchanged. The new test-only adapter binds the draft and expiry-boundary cases to RemoveChats, RestoreRemovedChat, DeleteRemovedChat, SaveUIState and the actual retention sweep. Restart creates a new App on the isolated home; it does not run the full background startup sequence.

Draft values are read directly from persisted ui-state.json. The provider-delete hook checks that the retained manifest still exists before native deletion and records the invented provider's actual outcome. The adapter reads live or archived transcript bytes and retained settings. It fails explicitly for unbound runtime, bus-routing, linked-record and lease observations. It also refuses to synthesize the expiry result required by the provider-outcome test from expected state.

The initial selected run used FIFO ticket 2176 after the priority document investigation. Draft preservation and stale-save cases for final deletion and expiry, plus the exact thirty-day boundary, passed in 1.954 seconds. The remaining startup counter recipe was requested from Engine in message 8764. Release's durable model, combined drawing authority and rendered/asset checks remain outside this partial host binding.

Provider deletion cases then passed in 10.131 seconds for supported, unsupported and failing deletion through Delete Now and Empty Bin. The frozen checks observed the retained archive at every supported/failed native callback, verified local erasure and tombstones, checked providerCopyRemains and its reason, and reopened the isolated home without resurrection. Provider state is an invented callback-owned copy; no native account or service was contacted. The expiry provider-result matrix remains unbound because the sweep has no returned result, although the separate expiry-boundary and stale-save cases ran.

Commands:

- `go test -tags quality_chat_cleanup,chatcleanup_acceptance ./cmd/dinah-desktop -run 'TestQualityCleanupDraftsAndStaleSave|TestQualityCleanupExpiryBoundary' -count=1 -timeout 60s`
- `go test -tags quality_chat_cleanup,chatcleanup_acceptance ./cmd/dinah-desktop -run '^TestQualityCleanupProviderDeletion/(supported|unsupported|failing)/(delete|empty)$' -count=1 -timeout 60s`

Both commands ran through scripts/with-test-lock.ps1. Engine confirmed the startup observation gap in message 8770 and is supplying counted launch adapters with a live control. Those frozen cases remain unbound. These results accept only the exercised host behaviors, not the combined cleanup product.
