# Bulk chat cleanup

Paul decided on 15 November 2026 that removal is local and restorable for thirty days. Staging never deletes an agent program's own copy. Paul's later decision enables documented provider deletion at final deletion only. The host-owned switch is on. This contract applies to the new cleanup page and the shared `RemoveChat` host operation. The architectural principles in `C:/Users/paul/OneDrive/Documents/Architectural Principles.md` apply.

## Page calls

The page uses these host calls. The listing is a query; removal and restore are person-only mutations and are absent from agent applet and bus catalogs. The page must retain row positions while selection or operation results change.

```go
func (a *App) ListChatCleanup() (ChatCleanupListing, error)
func (a *App) RemoveChats(ids []string) []ChatCleanupResult
func (a *App) RestoreRemovedChat(id string) ChatCleanupResult
func (a *App) DeleteRemovedChat(id string) ChatCleanupResult
func (a *App) EmptyChatRecycleBin() []ChatCleanupResult
```

The JSON listing has `rows` and `removed` arrays, `retentionDays: 30`, and `generatedAt` in RFC3339Nano UTC. It also has `providerDeletionEnabled` and authoritative `finalDeleteConfirmation` strings for destructive confirmation. The section is called Recycle Bin. Single-chat labels are Delete Chat; the batch label is Delete N Chats. The old Hide path no longer exists. Both arrays use the same row shape. Active rows sort by oldest known activity first, with unknown dates first, then session identity for equal dates. Removed rows sort by removal date, oldest first. Filters run on the returned rows and do not contact a provider. The snapshot contains all locally known chats across registered and previously used workspaces, including closed and hidden chats; it neither opens a session nor starts a provider to discover remote-only history.

```json
{
  "sessionId": "provider:identity",
  "title": "Planning",
  "workspaceId": "registered-id-or-empty",
  "workspace": "absolute-workspace-path",
  "workspaceName": "Display name",
  "provider": "codex",
  "providerName": "Codex",
  "lastActivity": "2026-11-01T10:00:00Z",
  "messageCount": 12,
  "messageCountKnown": true,
  "open": false,
  "working": false,
  "pinned": false,
  "eligible": true,
  "refusal": "",
  "removedAt": "",
  "expiresAt": "",
  "restorable": false,
  "restoreRefusal": "",
  "providerDeletion": "unknown",
  "finalDeleteConfirmation": "The program will be asked to delete its copy only if it offers documented deletion."
}
```

An unknown activity date is an empty string. An unknown message count is zero with `messageCountKnown: false`; the page must display it as unknown. The count comes from saved transcript user and assistant messages, without loading a chat into the runtime. Tool, status and delivery bookkeeping rows do not count as additional messages. A closed chat without a saved transcript remains listed if local settings or other saved metadata identify it. The host owns eligibility and rechecks it at removal time. Working, pinned, restoring, transferring, opening and shared-queue chats receive a plain refusal on their row, together with every existing `RemoveChat` refusal.

Every input identity produces one result in input order, even if another identity failed. Removal calls `RemoveChat` in turn. Repeated removal is idempotent. Restore returns a single result of the same shape.

```json
{
  "sessionId": "provider:identity",
  "ok": true,
  "status": "removed",
  "error": "",
  "row": {}
}
```

The host emits `chat:removed` with `{sessionId}` after removal has committed and runtime disposal has completed. Frontend listeners use the existing idempotent chat removal/disposal path to discard controllers, cached rows and notices. Restore emits `chat:restored` with `{sessionId}` after reactivation, without opening a chat. The page still applies each call result in place, so it does not depend on event arrival order.

The result status is `removed`, `restored`, `deleted`, `refused` or `failed`. `DeleteRemovedChat` permanently deletes one bin entry. `EmptyChatRecycleBin` snapshots the current bin and returns one result per identity in removal-date order, continuing after failures. Neither call accepts active chats or an agent-supplied provider-deletion policy. `row` is the authoritative current row when available, otherwise null. A refused or failed result always has a nonempty `error`. The frontend updates that identity's existing row in place, then may refresh the snapshot while preserving its current positions. Removed entries have `removedAt`, `expiresAt`, `restorable` and any `restoreRefusal`. Legacy permanent removals cannot be restored when their history was already discarded; they must never be presented as recoverable.

## Retention and runtime behavior

Removal keeps the transcript, chat settings and bus place on disk for thirty days. It closes the chat window, revokes capabilities and releases the application conversation state. It closes the provider session through documented individual unload where supported. The loaded-session exception below applies when that interface is absent. It never starts a provider to close a chat that is already closed. The retained chat is absent from ordinary lists, counts, Decisions and broadcasts. Startup does not load its transcript or start a provider for it. A bus send to it refuses with a removed-chat reason and creates no queued delivery; existing queued work is retained but cannot wake the chat.

Restore reactivates the same bus identity and settings and returns the chat closed. It does not open the chat or start a provider. Expiry disposes of the application's retained files and queue while keeping a small permanent removal marker so old provider history cannot silently return to the list. The provider-deletion switch is enabled. Final deletion uses only documented native deletion where supported; the immutable native identity is retained through interrupted finalization. The switch is host-owned and cannot be supplied by the page or an agent. Interrupted removal, restore and expiry must resume safely after restart, without duplicate queue ownership or reactivation of expired data.

The staging confirmation says: "The selected chats will move to the Recycle Bin now and can be restored for 30 days. They will receive no further messages. A session that its agent program cannot unload may stay loaded until that program stops or the window restarts. Their agent programs' own histories are untouched at this step." The product substitutes its display name where appropriate. The final confirmation follows the enabled policy below. Only an ID tombstone survives final deletion; no title, workspace, options, pins or transcript remains.

## Verification

Host tests cover oldest-first listing across workspaces, closed transcript counts without opening, authoritative refusals, mixed batch outcomes, restore of content and settings, retained bus identity, interrupted operations and expiry. Runtime-cost tests require zero new provider starts for listing, removal of closed chats, startup with retained removals and restore. Direct, group and broadcast sends cannot enqueue or wake retained removals. Removed askers and recipients are excluded from Decisions. Tests use invented homes and bus data.

The new bin behavior also applies to the compatibility host methods `HideSession` and `DeleteSession`, so an old page cannot bypass staging. Native deletion for finalization, if enabled, uses a separate internal operation. These compatibility methods are not a second deletion policy.

## Final deletion policy decided by Paul

The provider-deletion switch is enabled. Finalization first asks the provider to delete its copy through a documented interface, then erases the local retained copy. ACP deletion is used only when `sessionCapabilities.delete` is advertised. No implementation deletes files from an agent program's directories. A provider without documented deletion keeps its copy. Failure or lack of support never prevents completion of the application's local deletion.

Each row adds `providerDeletion` with `supported`, `unsupported` or `unknown`, and its own authoritative `finalDeleteConfirmation`. Unknown means the capability will be checked at finalization without starting that provider merely to list the bin. Unsupported rows explicitly say that the agent program's copy will remain. The listing's aggregate confirmation says: "This permanently deletes the selected chats from this application. Each agent program will be asked to delete its own copy where documented deletion is available. Unsupported or failed provider deletions leave that program's copy intact. This cannot be undone."

A deleted result additionally has `providerCopyRemains` and `providerDeletionReason`. A provider refusal produces `ok: true, status: deleted` once local deletion succeeds, with `providerCopyRemains: true` and the provider's reason. The frontend displays that reason beside the completed local deletion. A local-erasure failure instead returns `failed`, preserving the durable finalization state for retry. Provider deletion precedes local erasure even on retries; already-absent provider sessions count as deleted when its documented response establishes absence.

Stand-in tests must cover advertised deletion, absent capability and failed deletion, and prove that all three cases erase the local copy while only the advertised successful case removes the provider copy. Expiry uses the same finalization path as Delete Now and Empty Recycle Bin.


## Loaded sessions without individual unload

A loaded idle chat remains eligible when its agent program offers no documented individual unload. Binning closes its window and removes its application runtime conversation, ordinary listing entries, counts, Decisions and broadcast eligibility. No prompt, queued delivery, connection request or addressed bus message may reach it afterwards. The host must not stop unrelated chats to unload it. Its provider session can remain loaded until that provider has no other active chats, or until the next window restart, whichever occurs first. The application releases the provider process when no other chat needs it. A restart never reloads a binned session.

Each bin row has `runtimeNote`, an authoritative string which is empty when no loaded session remains. Otherwise it names the program and says, for example, "Still loaded by Codex until the program stops or the window restarts." This note is an explicit exception to immediate provider-memory release; the retained chat still cannot receive work. Listing this note does not start or reconnect a provider.

Restore reactivates the retained identities but leaves every session closed. Previously retained deliveries remain dormant until the person explicitly opens the chat or a new accepted message addresses it. Restore itself neither sends a prompt nor starts a provider.

## One chat includes its switched session chain

Cleanup groups the connected session chain into one logical row. The host includes edges from `ChatBindings`, `ChatTransfers`, `ChatTransferHistory` and saved transcript parent links. It includes allocated targets of unfinished or rejected switches, including targets recorded only in history. It does not treat an uncommitted target as the current display session. A committed current head supplies the row identity; a stable identity breaks ambiguous ties. Supplying any member identity to a mutation resolves the entire chain.

Binning stages every member together. Eligibility covers every member, so a working or pinned member refuses the whole operation. Bus ownership shared only by members of the same chain is retained once; ownership outside the chain refuses removal. Restore restores the entire chain and its links while keeping all members closed. Final deletion asks each program to delete its native session through its documented interface, oldest session first, then erases the application's chain. Unsupported or failed native deletion does not stop the remaining members or local erasure.

Rows add `sessionIds`, an array of all member identities in oldest-first order, and `programCopies`, an array with the following JSON shape:

```json
{
  "sessionId": "provider:identity",
  "provider": "codex",
  "providerName": "Codex",
  "deletion": "unsupported",
  "copyRemains": true,
  "reason": "This program offers no documented deletion."
}
```

The `deletion` value is `supported`, `unsupported` or `unknown`. Before final deletion, `copyRemains` is true because staging preserves every provider copy. The row's authoritative confirmation identifies each known unsupported program. Final results also include `programCopies` with the observed result for every member. `providerCopyRemains` is true when any copy remains, and `providerDeletionReason` summarizes those reasons. The existing result identity remains the input identity, so the page can update its original row in place. Lifecycle events are emitted for each physical session identity so existing frontend controllers are disposed of.

## Independent acceptance bindings

The implementation handover must include test-only bindings, compiled with the `chatcleanup_acceptance` build tag in the desktop package. They must not add a product route, applet action or bus tool. The binding will use these package-local Go shapes, available to same-package acceptance tests:

```go
type chatCleanupAcceptanceHooks struct {
    Now func() time.Time
    DeleteProvider func(context.Context, string, string) (supported bool, err error)
    UnloadProvider func(context.Context, string, string) (supported bool, err error)
}

func (a *App) chatCleanupAcceptanceInstall(h chatCleanupAcceptanceHooks) (restore func())
func (a *App) chatCleanupAcceptanceSweep() error
func (a *App) chatCleanupAcceptanceObserve(id string) (chatCleanupAcceptanceState, error)

type chatCleanupAcceptanceState struct {
    SessionIDs []string
    Binned bool
    Tombstoned bool
    TranscriptPresent bool
    SettingsPresent bool
    QueuePresent bool
    BusActive bool
    RuntimeLoaded bool
    ProviderLoaded bool
    RemovedAt time.Time
    ExpiresAt time.Time
}
```

The provider callbacks receive provider identity and native session identity, in that order. A false supported result simulates an absent documented capability; a true result with an error simulates a failed documented operation. The install function returns a cleanup function which restores the prior hooks. Nil callbacks retain production behavior. Tests may advance a captured clock and call Sweep without sleeping. Sweep runs the same expiry and interrupted-operation reconciliation used by the product.

Observe reads durable artifacts and runtime ownership without opening a chat, starting a provider or mutating state. For a chain, the boolean presence fields are true when any member has that state; SessionIDs exposes the complete group for per-member observations. Tombstoned is true only when every member retains only its permanent removal identity. Tests also inspect the isolated home directly to prove that titles, options, pins and transcript bytes are absent after final deletion. The handover supplies the isolated App fixture setup and shutdown, plus stand-in provider start, prompt and unload counters, so zero starts and zero wakeups can be asserted independently.

Required chain coverage includes three agent programs, one without deletion, and history-only allocated targets. Loaded-session coverage bins a session without individual unload, attempts direct and queued input, restarts on the same home and proves that it is never reloaded. Provider deletion coverage includes successful deletion, missing capability and failure, with provider-first ordering observed before local erasure. These bindings are a handover requirement, not a claim that implementation or execution has completed.


## Retained composer drafts and stale page writes

Staging retains the composer draft stored in `ui-state.json` under `dinah-draft:chat:<identity>` for every member of the switched chain. Restore preserves those drafts. Final deletion and expiry erase those values along with the other local data. The host serializes finalization with UI-state saves and filters permanently tombstoned chat identities from every subsequent save, so a stale full-state write cannot revive an erased draft. The independent acceptance fixture must exercise a stale save after final deletion and after expiry, including a draft for an earlier switched session. Frontend removal events discard cached rows without provider discovery; Restore applies the authoritative returned row to the closed-chat cache without calling Sessions or SessionsAt.


## Ownership and runtime authority

Binning, Restore and final deletion affect only the chat's own records and runtime state, including every session in its switched chain, and the agent program's copy through its documented delete. A reference from a chat does not confer ownership of the referenced record. Cleanup never changes or deletes documents, document drafts, comments, suggestions, attachments, publication records, workspace files, remote-service data, credentials or tool registrations. The composer text described above is chat-owned UI state; it is distinct from a document draft.

Entering the bin revokes every runtime lease and capability held by every chain member, including drawing leases. It cancels pending work authorized through those leases and prevents any later completion from exercising revoked authority. Restore cannot reactivate a previous lease. The next explicit open or accepted new message creates a fresh runtime and fresh leases through the ordinary documented opening path.

Acceptance seeds a switched chat which refers to a document and a document draft. It bins, restores and finally deletes that chain, then compares the referenced bytes and records with their original values and asserts zero remote writes. The test also attempts to use the old drawing lease after binning and after Restore; both attempts must refuse. An open after Restore obtains new authority without making the old lease valid. The isolated handover must expose the existing lease service through a test-only binding when needed, without adding a product route.
