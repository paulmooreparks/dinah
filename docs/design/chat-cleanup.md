# Bulk chat cleanup

Paul decided on 15 November 2026 that removal is local and restorable for thirty days. It never deletes an agent program's own copy, including Devin's copy. This contract applies to the new cleanup page and the shared `RemoveChat` host operation. The architectural principles in `C:/Users/paul/OneDrive/Documents/Architectural Principles.md` apply.

## Page calls

The page uses these host calls. The listing is a query; removal and restore are person-only mutations and are absent from agent applet and bus catalogs. The page must retain row positions while selection or operation results change.

```go
func (a *App) ListChatCleanup() (ChatCleanupListing, error)
func (a *App) RemoveChats(ids []string) []ChatCleanupResult
func (a *App) RestoreRemovedChat(id string) ChatCleanupResult
```

The JSON listing has `rows` and `removed` arrays, `retentionDays: 30`, and `generatedAt` in RFC3339Nano UTC. Both arrays use the same row shape. Active rows sort by oldest known activity first, with unknown dates first, then session identity for equal dates. Removed rows sort by removal date, oldest first. Filters run on the returned rows and do not contact a provider. The snapshot contains all locally known chats across registered and previously used workspaces, including closed and hidden chats; it neither opens a session nor starts a provider to discover remote-only history.

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

The result status is `removed`, `restored`, `refused` or `failed`. `row` is the authoritative current row when available, otherwise null. A refused or failed result always has a nonempty `error`. The frontend updates that identity's existing row in place, then may refresh the snapshot while preserving its current positions. Removed entries have `removedAt`, `expiresAt`, `restorable` and any `restoreRefusal`. Legacy permanent removals cannot be restored when their history was already discarded; they must never be presented as recoverable.

## Retention and runtime behavior

Removal keeps the transcript, chat settings and bus place on disk for thirty days. It unloads the conversation, closes its provider session where supported, revokes capabilities and releases its in-memory state. It never starts a provider to close a chat that is already closed. The retained chat is absent from ordinary lists, counts, Decisions and broadcasts. Startup does not load its transcript or start a provider for it. A bus send to it refuses with a removed-chat reason and creates no queued delivery; existing queued work is retained but cannot wake the chat.

Restore reactivates the same bus identity and settings and returns the chat closed. It does not open the chat or start a provider. Expiry disposes of the application's retained files and queue while keeping a small permanent removal marker so old provider history cannot silently return to the list. Expiry never invokes provider deletion. Interrupted removal, restore and expiry must resume safely after restart, without duplicate queue ownership or reactivation of expired data.

The confirmation says: "The selected chats will be removed from this application now. You can restore them for 30 days. The agent programs' own histories are untouched." The product substitutes its display name where appropriate. No provider-specific confirmation may promise deletion from that provider.

## Verification

Host tests cover oldest-first listing across workspaces, closed transcript counts without opening, authoritative refusals, mixed batch outcomes, restore of content and settings, retained bus identity, interrupted operations and expiry. Runtime-cost tests require zero new provider starts for listing, removal of closed chats, startup with retained removals and restore. Direct, group and broadcast sends cannot enqueue or wake retained removals. Removed askers and recipients are excluded from Decisions. Tests use invented homes and bus data.
