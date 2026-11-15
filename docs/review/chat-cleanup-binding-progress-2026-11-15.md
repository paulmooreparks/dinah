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

## Integration gate decision

Team Lead's message 8821 makes Engine's interim logicalChatMembers query the acceptance boundary for this cleanup release. It resolves bindings, transfers and history. Release's later durable model replaces that function internally and is not a prerequisite for cleanup. The earlier reference to waiting for Release's model is superseded.

Quality will run the following gates when Engine supplies the immutable combined candidate and bindings:

- A switched chat linked to a document and document draft must leave those records unchanged through binning, Restore and final deletion. Remote writes must remain zero, and the old drawing lease must be refused.
- Working, pinned, allocation-in-progress and independently owned descendant cases must refuse the prohibited cleanup operation.
- The interim member query must move, restore and delete the owned chain together while leaving a fork and a workspace move untouched.

The frozen assertions remain unchanged. Additional binding requirements and any new assertions for allocation or descendant refusal will be recorded explicitly. No acceptance is inferred from the choice of interim implementation. Quality awaits Engine's bindings and retains these gates; no test was queued for this documentation change.

## Corrected combined candidate checks

Quality merged the complete interim-resolver candidate and then the standalone Codex deletion guard and its damaged-transcript correction. The product source matches e4ae7b4145ffe7b229e2a44133fad28ab1556d5c. Quality's additional files contain only acceptance assertions and bindings. The original frozen assertions remain unchanged.

Before the guard correction arrived, the working/pinned refusal, protected history member, mixed batch, switched-chain and separate-recipient cases passed on 974bf516 in 5.541 seconds through ticket 2203. That run is bounded to its earlier source. The later binding gives the workspace-move fixture its own directory rather than sharing the source workspace.

The added allocation assertion reserves a live transfer before any runtime exists and checks refusal without a launch. The independent-descendant assertion connects the real Codex adapter and host guard to an invented subprocess, observes thread/delete requests, and checks both a separate recipient and damaged transcript-only identity. Its allowed owned-descendant control must produce a native delete. Cleanup's documented-deletion callback feeds the adapter refusal into the actual finalization result; local cleanup must report that the provider copy remains while preserving the independent descendant.

The linked-record binding opens and edits a real local document draft and records transcript references to it and its document. It compares the workspace file, Recent Documents index, workspace record and persisted draft JSON across the frozen lifecycle. It opens the stand-in provider explicitly and obtains a real managed drawing lease, probes authority before binning, starts a request that reaches a substituted renderer, checks cancellation and old-token refusal, and checks fresh authority after explicit reopening. Rendering is substituted; native page disposal is outside this test. An HTTP write observer has a positive control against a local fake server. The content fixture is local, so it does not exercise a remote document provider or real credentials.

Tickets 2212 and 2214 were withdrawn before execution while completing the bindings and protocol headers. Ticket 2215 is the selected combined run. Its final binding source before execution is 4a3cbf6e. Results are pending; no integration acceptance follows from this preparation.

### Final scoped result

The first combined selected run failed in 7.816 seconds only at the post-restart workspace-record comparison. A focused evidence run reproduced that failure in 1.224 seconds. The retained snapshots in cleanup-record-evidence-20261115 show that both mountAlias fields were empty before restart and became 003 on restart. Quality had constructed the workspace record without the normal ensureWorkspaceMountAliases registration step. Document bytes, the Recent Documents record and the document draft did not change. The corrected seed calls that production registration helper before recording the baseline. No frozen assertion or product source changed.

On the corrected seed, all eight selected groups passed in 7.123 seconds. They covered the linked-record/drawing lifecycle, allocation-in-progress refusal, independent-descendant deletion refusal with an allowed-delete control, working/pinned refusals, protected history members, mixed batches, whole switched chains and separate forks/workspace moves. The normalized snapshots are retained in cleanup-record-evidence-20261115-normalized. The HTTP mutation observer remained at zero after its positive control. No picture bytes returned from cancelled work. Old authority failed after binning and Restore; explicit reopening obtained fresh valid authority, which the second removal revoked.

The earlier dormancy/counted positive control, draft/stale-save, exact expiry boundary and six provider-delete outcomes were then rerun on the same corrected combined product source and passed in 5.434 seconds. All runs used scripts/with-test-lock.ps1. No runner remains active or queued.

Quality accepts the three integration gates specified in message 8821 on product candidate e4ae7b4145ffe7b229e2a44133fad28ab1556d5c, within the documented invented-provider, local-document and substituted-renderer boundaries. The independently owned descendant test invokes the real Codex adapter through the documented-deletion fixture callback; it does not perform an account-backed native deletion. Engine still owns the full package/vet gate and the combined handover. This result authorizes no install or merge from Quality. The loaded-without-unload frozen case and expiry provider-result matrix remain outside this selected binding; their absence is not presented as a pass.

Selected command:

`go test -tags quality_chat_cleanup,chatcleanup_acceptance ./cmd/dinah-desktop -run '^TestQualityCleanup(OwnershipAndLeases|AllocationRefusal|IndependentDescendant|SwitchedChain|SeparateRecipients|Refusals|ProtectedChain|MixedBatch)$' -count=1 -timeout 90s`

The preceding counted-startup command in this record was used for the final regression rerun. The original frozen assertion file remains byte-for-byte unchanged from d4eba107.

## Remaining promised cases

The loaded-without-unload case and all three expiry provider outcomes now pass on unchanged product source e4ae7b4145ffe7b229e2a44133fad28ab1556d5c. The original frozen assertion file is unchanged from d4eba107.

The loaded fixture assigns the invented frozen identity to the counted Codex stand-in and seeds its open runtime directly. A second invented conversation explicitly starts and loads that provider, so binning the target leaves the shared connection alive. The additional delivery assertion reads the actual removed inventory's still-loaded text, attempts direct bus delivery, and requires refusal with unchanged prompt and window-request counters. Restart stops the old lifecycle and joins turns before reconstructing App and running reconciliation, queue restoration, ChatWindows and bus discovery. No additional launch or load occurs, and the binned chat has no loaded provider after reconstruction. This remains a simulated application restart with invented connections, not a native desktop restart.

Expiry now observes the actual ChatCleanupResult emitted by chat:finalized during the real sweep. The wrapper delegates to the prior sink to preserve connection counters. Supported, unsupported and failing deletion at the exact 30-day boundary all pass, including remaining-copy reasons, native-before-local deletion ordering where supported, tombstones and no resurrection. No result is synthesized from expected state.

The first command accidentally left the comma-separated build tags unquoted in PowerShell, which split the tags and did not compile the binding. The corrected command ran the matrix but failed the loaded seed because OpenSessionAt correctly rejected its fixture-prefixed identity with a saved Codex provider. The seed was corrected to use the runtime fixture recipe while explicitly opening the valid second conversation. The focused frozen cases then passed in 4.930 seconds. These were invocation and fixture failures; no product source or frozen assertion was changed.

The complete independent cleanup selection, including the additional loaded-delivery probe and the counted positive control, passed in 14.381 seconds through scripts/with-test-lock.ps1:

`go test -tags 'quality_chat_cleanup,chatcleanup_acceptance' ./cmd/dinah-desktop -run '^TestQualityCleanup|^TestCleanupAcceptanceRestartCountsRealProviderEntry$' -count=1 -timeout=90s`

This closes Quality's remaining gates from message 8855. Engine separately reported the full corrected-source package and vet gate in message 8858. Engine retains the combined handover; Quality has no active or queued run. No real home, account, remote write, native provider deletion, installation or merge to a product branch occurred.
