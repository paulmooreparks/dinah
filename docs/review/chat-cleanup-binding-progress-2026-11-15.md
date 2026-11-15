# Cleanup host binding progress

Quality integrated candidate 61dd1e66 into quality/chat-cleanup without changing product source. The frozen assertions remain unchanged. The new test-only adapter binds the draft and expiry-boundary cases to RemoveChats, RestoreRemovedChat, DeleteRemovedChat, SaveUIState and the actual retention sweep. Restart creates a new App on the isolated home; it does not run the full background startup sequence.

Draft values are read directly from persisted ui-state.json. The provider-delete hook checks that the retained manifest still exists before native deletion and records the invented provider's actual outcome. The adapter reads live or archived transcript bytes and retained settings. It fails explicitly for unbound runtime, bus-routing, linked-record and lease observations. It also refuses to synthesize the expiry result required by the provider-outcome test from expected state.

The initial selected run is queued under FIFO ticket 2176 behind the priority document investigation. No pass or failure is available yet. The remaining startup counter recipe was requested from Engine in message 8764. Release's durable model, combined drawing authority and rendered/asset checks remain outside this partial host binding.
