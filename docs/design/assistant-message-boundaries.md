# Assistant message identity and completion

Engine owns adapter and saved-capture diagnosis for the incident reported in bus 2457. This worktree starts at released `40d133a`, independently of accepted resume candidate `c90f5db` and the frontend toolbar candidates. The incident evidence is preserved read-only under `.diagnostics/interlaced-originals/`.

## Findings

The uniquely resolved Team Lead chat is `codex:01a18fea-f631-7ff1-bc55-c99ef167949b`. The original saved snapshot contains the old prefix at message 1180, intervening delivered notices at 1181 and 1182, and a single agent row at 1183 containing the old suffix concatenated with the newer commentary. The screenshot's corruption therefore exists in saved capture as well as presentation.

The preserved native rollout contains the newer commentary as a separate complete item, `msg_01df20d13a3c64ea016ae66736323881919c527d4116a60340`, in turn `01a19402-a17c-7300-aa1f-365809312f8b`. That turn starts at 22:09:22.845 UTC, receives a steered user message at 22:09:49.669, records the newer completed item at 22:09:56.624 and finishes at 22:10:09.220. There is no recorded turn-level cancellation in that interval. The earlier partial phrase has no matching completed item in the preserved rollout. Full delta bodies were not retained by the host's header-only Codex trace. These records cannot establish whether the provider abandoned an unfinished item during steering or whether some earlier event was lost. No intended sentence is reconstructed.

The adapter parses itemId but discards it when emitting text chunks and does not parse their original turnId. It suppresses completed message text once any delta for the item streamed. Saved capture merges adjacent agent chunks by role alone. These are concrete defects even though the complete original delta ordering is unavailable.

The synthetic adapter-to-capture probe supplies an old partial, an intervening delivered notice, a late old-item delta, a distinct new item in the same turn, and an authoritative new-item completion. On unchanged released source it fails the required separation assertion. Its actual agent rows are retained in `.diagnostics/boundary-probe-result.json`; the raw Go JSON failure is retained alongside it. The probe source is diagnostic evidence, not a default product test that accepts broken behavior.

## Shared additive update contract

Identified text updates retain the existing sessionUpdate and content fields and add these top-level fields:

```json
{
  "sessionUpdate": "agent_message_chunk",
  "turnId": "original-provider-turn",
  "itemId": "original-provider-item",
  "contentMode": "append",
  "content": { "type": "text", "text": "a fragment" }
}
```

The contentMode value is either `append` for a delta or `snapshot` for the authoritative completed/replayed item. Snapshot text may be empty. The same envelope applies to agent_thought_chunk for reasoning; the text category remains part of the accumulator key. The key is scoped to its conversation and consists of category, original turnId and original itemId. A team notice never changes that key. Separate items in the same turn remain separate messages.

A snapshot replaces only its own item and finalizes it. Repeated identical snapshots are idempotent. A later append for a finalized key is explicitly ignored; it must never reopen the item or enter another accumulator. Another authoritative snapshot for that same key replaces its prior snapshot. Replay uses the enclosing original turn ID supplied by the resumed history, never the session's currently active turn. Completion emits a snapshot even when earlier deltas streamed.

Both identities and a valid contentMode must be present to enter the identified path. Missing-ID and legacy-provider updates retain the existing legacy append behavior, with no guessed identity and no ability to replace an identified item. The adapter's missing-ID compatibility path must avoid emitting an unaddressable replacement. Existing saved files remain readable and unchanged; no migration guesses their lost boundaries.

Newly captured identified saved rows add optional turnId, itemId and messageComplete fields. Capture retains one accumulator per retained identified row and updates that row when late fragments belong to it. Capture continues its existing omission of thought content; thought updates must never enter an assistant row. The frontend keeps reasoning and visible assistant accumulators separate. Reconstructed partial text is redacted as a whole before persistence. Authoritative snapshots are redacted independently before replacement, including empty text. Raw reconstruction state is released on completion or lifecycle retirement; bookkeeping is scoped to the retained conversation/replay, not an indefinitely growing process-global tombstone set.

The official [Codex App Server contract](https://learn.chatgpt.com/docs/app-server#item-deltas), read on 1 November 2026, identifies item/started IDs with the itemId used by deltas and defines item/completed as authoritative. It also documents that steering adds input within an active turn. No correction may depend on sentence punctuation, a delivery notice acting as a boundary, or inferred provider cancellation.

## Required verification

Regression coverage must compare live updates, saved rows and replay for same-turn distinct items, distinct turns, late old-item fragments, reasoning versus assistant output, authoritative empty and nonempty snapshots, repeated completion and post-completion deltas. It must preserve legacy missing-ID behavior and demonstrate reconstructed-fragment redaction. Front End owns rendered verification with the same contract. Quality independently reviews the immutable correction. Existing snapshots are never rewritten to manufacture missing identities.

Engine releases the shared test slot after the retained negative probe. Product implementation and combined release expansion are not claimed by this diagnosis record.
