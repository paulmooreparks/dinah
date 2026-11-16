# Chat inventory consistency boundary

This work answers Lead's message 10082 on a separate branch from shipped master 99fe1f00. The seven-source settlement work remains on its own branch. No page changes, merge, installation or live-state writes belong to this correction.

## Cause and correction

SavedChats compared the entire Settings value before and after its read. Consequently, changing a preference, a mention cursor, provider options, window settings or document settings could produce an identity refusal. Its runtime comparison included working state, queues, stream positions, background work, state reasons and team counts. Those fields change during ordinary turns and scheduled state publications. The two earlier synchronous publication fixes cover Start Chat and removal, but cannot make continuously changing execution information into stable ownership.

OrgChart and ListChatCleanup each read SavedChats twice and compared its complete display generation. Even if each individual inventory read succeeded, a state or activity update between them caused a refusal. This is a separate source of false invalidation.

The correction validates explicit ownership and repair metadata separately from captured display information. Runtime identity consists of the runtime key, session ID, provider, workspace and title. Saved settings validation retains listing identity and repair evidence, titles, provider ownership, canonical recipient mappings, bindings, transfers, removal reservations, hidden identities and workspace metadata. Listing observation timestamps, freshness and sequence do not establish ownership. Transcript identity headers and recipient title/retirement observations remain guarded.

The public generation continues to describe the captured display result. A private identity stamp lets the chart and cleanup validate their ownership boundary across complete inventory reads. Execution state and timestamps remain in their captured rows, and the existing ordered state events continue to update the page. No query retries or mutation replays are added. Existing refusal wording remains unchanged for Front End's bounded read recovery.

## Caller audit

| Reader | Entry points and consumers |
| --- | --- |
| SavedChats | Chats panel inventory refresh and group chat catalog via chat-browser.js; direct page API; RefreshProviderSessions after explicit native discovery |
| ListChatCleanup | Cleanup page load and authority refresh; chat-browser.js restore listener; RemoveChats preflight; final deletion preflight; restore follow-up listing |
| OrgChart | Chart page; OrgChartActions and its mutation preflight; task host inspection |
| OrgChartActions | Actions page load and refresh; ApplyOrgChartAction revalidation and recovery choices |
| Sessions and SessionsAt | Native listing lifecycle, separate from this guard; explicit discovery can subsequently return SavedChats |

The direct toast path is chat-browser.js refreshSessionInventory. Its catch calls toast with danger severity. The restore listener calls ListChatCleanup and forwards failure to fail. Explicit provider refresh forwards rejected RefreshProviderSessions results to fail. The chart itself presents a local status, although its optional inventory refresh also reaches the shared Chats panel reader. Front End owns recovery for these paths under message 10082 and declared three total read attempts in 10090.

## Write audit

The old full-settings stamp included every updateSettings caller. The new projection excludes preferences, consent grants, provider configuration catalogs and defaults, option values, mention migration/cursors, editor and window choices, recency and listing-scope bookkeeping. These writes cannot change the guarded projection.

Runtime publication through EmitSession, sendChatStates and sendTeamStates can change execution, queue, stream, history-loading, window and team-count observations. Turn completion, permission changes, queued messages, background work and parking therefore no longer masquerade as identity changes. Actual runtime admission/removal, title, workspace and program changes remain observable identity changes.

Identity-changing writes remain subject to refusal. They include chat creation and reopening, rename and judged-title publication, provider listing identity or repair changes, continuation and provider/workspace transfers, canonical recipient registration, bin reservation/restore/final deletion, hidden-chat changes and transcript header creation/deletion. Bus rename and retirement remain guarded separately. A real concurrent ownership mutation still requires a fresh read; the host does not conceal it.

## The 08:46 notification

Read-only inspection of the installed Dinah notification store found an app/error item created at 2026-11-16T00:46:46.5141799Z. Its title is the exact SavedChats refusal, and its body, session ID and source ID are empty. The UI log independently records the same error at 08:46:46.547 +08:00. Neither records the originating page or failing comparison. The shared Chats panel inventory-refresh toast is the most likely source by code inspection; the saved evidence cannot prove that attribution or identify the concurrent writer. No private surrounding messages are copied into this document.

## Verification status

Deterministic tests schedule unrelated settings writes and actual runtime state publication inside the SavedChats read and between the chart/cleanup inventories. The publication control first proves that the full runtime snapshot changed. Ownership controls retain title, provider, recipient, binding, hidden, workspace and repair changes, alongside the existing real concurrent rename refusal.

The first two producer commands exposed an uninitialized ListedChats map in the new metadata-control fixture. The execution-publication and unrelated-settings cases passed in those runs; the metadata fixture was corrected without changing production validation. The first JSON receipt is retained at the private temporary path dinah-chat-inventory-race.jsonl.

The corrected initial selection passed ten race/all-vet repetitions in 38.688 seconds. The final expanded selection passed ten repetitions in 76.668 seconds, including SavedChats, the new boundary controls, bin publication, Start Chat and new-chat publication tests. Its private receipt is dinah-chat-inventory-final-race.jsonl.

## Cleanup receiving ports

The existing org_chart_acceptance/startup_page_acceptance fixture now exposes OrgChartHostFixtureCleanup, which executes the actual listing body. The control cleanup-identity-changes accepts count zero through three and an optional fixture chatId. It schedules actual RenameSession writes after the initial inventory and before final validation. Each affected call returns the unmodified production refusal, "Chat inventory changed while cleanup was read. Refresh the list." The next call succeeds when the scheduled writes are exhausted. cleanup-read-observations returns raw reads, writes and remaining counts. Rearming resets read counts but keeps the write serial monotonic so every scheduled title is distinct.

The first control run revealed that rearming reused the prior title, making that write a no-op. That receiving-only defect was corrected. The unchanged actual-refusal checks then passed ten race/all-vet repetitions in 28.616 seconds. Both receipts are retained privately as dinah-cleanup-identity-control.jsonl and dinah-cleanup-identity-control-fixed.jsonl.

The control cleanup-store-unavailable closes the actual isolated bus database handle. A subsequent listing fails through database/sql, rather than through a substituted JavaScript response. The existing restart control shuts down the old App and reopens the same persisted records. A subsequent OrgChartHostFixtureCleanup succeeds. The store-failure/restart test, actual-identity control and existing swap/restart test pass together with race/all-vet in 7.752 seconds, including protected-record comparison. The receipt is dinah-cleanup-store-control.jsonl. These controls exist only in the tagged fixture.

The full desktop package passes with all analyzers and no race instrumentation in 303.756 seconds, exit zero. Its private receipt is dinah-chat-inventory-full.jsonl. The separate negative control restores only the old full-runtime comparison through a Go overlay, leaving this worktree unchanged. It fails the execution-publication regression at the actual SavedChats refusal in 1.129 seconds, proving that the test detects the prior defect. Its receipt is dinah-inventory-old-guard-control.jsonl. Both commands used the canonical shared runner.

Quality's bounded source review in message 10132 found no blocking ownership-boundary defect in a4124bc9, without running a build or test. Independent runtime receiving, page behavior and installed acceptance remain separate. No owned runner remains for this branch. The production and fixture sources have not changed since a4124bc9.
