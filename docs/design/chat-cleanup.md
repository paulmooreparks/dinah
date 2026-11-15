# Bulk chat cleanup

Paul decided on 15 November 2026 that removal is local and restorable for thirty days. Staging never deletes an agent program's own copy. Whether final deletion also removes a provider copy is pending Paul's decision; that step is behind a host-owned switch, currently off. This contract applies to the new cleanup page and the shared `RemoveChat` host operation. The architectural principles in `C:/Users/paul/OneDrive/Documents/Architectural Principles.md` apply.

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
  "restoreRefusal": ""
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

Removal keeps the transcript, chat settings and bus place on disk for thirty days. It unloads the conversation, closes its provider session where supported, revokes capabilities and releases its in-memory state. It never starts a provider to close a chat that is already closed. The retained chat is absent from ordinary lists, counts, Decisions and broadcasts. Startup does not load its transcript or start a provider for it. A bus send to it refuses with a removed-chat reason and creates no queued delivery; existing queued work is retained but cannot wake the chat.

Restore reactivates the same bus identity and settings and returns the chat closed. It does not open the chat or start a provider. Expiry disposes of the application's retained files and queue while keeping a small permanent removal marker so old provider history cannot silently return to the list. With the provider-deletion switch off, final deletion never invokes a provider. If Paul enables it, only documented native deletion is used where supported; the immutable native identity is retained through interrupted finalization. The switch is host-owned and cannot be supplied by the page or an agent. Interrupted removal, restore and expiry must resume safely after restart, without duplicate queue ownership or reactivation of expired data.

The staging confirmation says: "The selected chats will move to the Recycle Bin now. They will stop running and can be restored for 30 days. Their agent programs' own histories are untouched at this step." The product substitutes its display name where appropriate. While the switch is off, final confirmation says: "This permanently deletes the selected chats from this application, including their saved messages, settings and queued bus messages. This cannot be undone. Their agent programs' own histories are untouched." The host returns that text as `finalDeleteConfirmation`. Enabling provider deletion must update the authoritative text to identify supported native deletion and leave unsupported providers explicit. Only an ID tombstone survives final deletion; no title, workspace, options, pins or transcript remains.

## Verification

Host tests cover oldest-first listing across workspaces, closed transcript counts without opening, authoritative refusals, mixed batch outcomes, restore of content and settings, retained bus identity, interrupted operations and expiry. Runtime-cost tests require zero new provider starts for listing, removal of closed chats, startup with retained removals and restore. Direct, group and broadcast sends cannot enqueue or wake retained removals. Removed askers and recipients are excluded from Decisions. Tests use invented homes and bus data.

The new bin behavior also applies to the compatibility host methods `HideSession` and `DeleteSession`, so an old page cannot bypass staging. Native deletion for finalization, if enabled, uses a separate internal operation. These compatibility methods are not a second deletion policy.
