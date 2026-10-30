# Steering active chats

Paul approved extending Claude's steering behavior to Codex, Ollama and Devin on 24 October 2026. This design follows the shared [Architectural Principles](C:/Users/paul/OneDrive/Documents/Architectural%20Principles.md).

## Shared behavior

Ordinary keyboard and implicit form submissions steer active work when the selected provider and model report support. Otherwise those submissions queue. The explicit Queue and Steer controls retain their named action. Hook delivery is eligible only after session binding, and attachments require a native delivery path.

Steer adds guidance while preserving active work. Stop and Steer cancels the turn before sending a replacement prompt. An adapter that cannot deliver a steer during the active turn leaves the input queued for a later turn, including its attachments. A new steer never silently selects the cancellation path.

Each submission has an identity and visible state. Pending means that the input awaits a delivery boundary. Accepted means that a native runtime acknowledged the input; it does not establish that the model consumed it. Delivered means that Dinah inserted the input into Ollama's history or handed it to a hook. Queued means that the input awaits a later turn, while sent means that the later turn has begun. A failed submission whose delivery cannot be confirmed remains visible as unconfirmed and is never retried automatically.

The host serializes steering submission with turn settlement. A submission that reaches the host after a turn finishes is sent through the ordinary idle-chat path. A submission accepted into an adapter's local queue must either receive a delivery callback before that turn ends or return to host ownership for later delivery. The adapter must not retain an undelivered copy for another turn.

## Provider mechanisms

| Provider | Mechanism | Boundary |
|---|---|---|
| Claude | Existing bus queue and hosted hooks | PostToolUse adds context; Stop supplies follow-up instructions. |
| Codex | Native app-server `turn/steer` | The request includes the expected active turn ID and preserves the turn. |
| Ollama | Dinah's local steering queue | Dinah inserts user messages after a complete model response and its tool results, before the next model request. |
| Devin | Command hooks connected to the bus queue | PostToolUse adds context; Stop supplies follow-up instructions. |

Codex and Ollama implement the internal `acp.Steerer` capability. The capability is an in-process interface and introduces no invented ACP wire method. Claude and Devin share the existing hook delivery implementation. A chat's hooks must bind the agent session to the window's bus session before the host uses hook delivery.

Codex's expected turn ID prevents a late request from changing a different turn. A lost or failed RPC response is reported without an automatic resend because the host cannot establish whether the runtime accepted the input. The existing prompt conversion handles text, images and supported embedded context for both ordinary prompts and steers.

Ollama's running generation and tools finish normally. Dinah then appends queued guidance to the conversation and persists it before reporting delivery. A steer arriving during a final response causes another model exchange in the same host turn. Input cannot be accepted after the adapter closes that turn's steering boundary. Existing round limits still apply; insertion into history does not promise another exchange if a limit or cancellation ends the turn.

Devin's project hooks file receives an idempotent command entry for UserPromptSubmit, PostToolUse and Stop, preserving existing hook entries. The installed `dinah steering-hook` command reads documented hook input and emits documented hook output. The Devin subprocess receives Dinah's state directory through DINAH_HOME so that the command opens the same bus database. Devin hook session IDs are namespaced separately from Claude's IDs. The installed dinah command must be on PATH, as required by the normal installation instructions.

Attachments use native delivery where the adapter and model support their content. The host checks image support before choosing automatic delivery. Codex uses the selected model's documented inputModalities catalog field, and Ollama uses its model capabilities. Missing attachment capability information selects Queue. Hook paths cannot carry attachments, so those submissions stay queued for the next turn.

## Provider changes during active work

Changing providers remains unavailable while a chat is working, reconnecting, or holding queued messages, for every provider. Session snapshots returned when a chat opens update the same working and queue state as live events. Reopening a running chat must not temporarily enable the provider dropdown while waiting for another turn-state event. Partial metadata updates preserve the last known working state.

## Verification

Protocol tests exercise Codex's expected turn ID, native acceptance and rejected requests. Ollama integration tests hold a generation open, submit steering, and verify ordering after both tool results and final responses, including persistence without duplication. Host tests cover accepted delivery, pending delivery, unavailable adapters, turn-boundary fallback and ambiguous errors without cancellation. Hook tests exercise Devin's session binding, context output, Stop continuation and single delivery. Registration tests preserve existing hooks and reject malformed input. The browser suite checks pending, accepted, delivered, queued, sent and unconfirmed labels.

The fixture tests use controlled protocols. An additional opt-in live test passed against the installed Devin ACP runtime on 24 October 2026: UserPromptSubmit bound the session, the queued guidance reached a lifecycle hook, and the same prompt response included a token supplied only through that guidance. The live test uses an isolated workspace and state database. The installed Codex CLI 0.162.0 also exports the expected turn/steer schema, including expectedTurnId. Codex and Ollama steering have fixture coverage; no live model test is claimed for them.

## External contracts

- [Codex app-server steering](https://learn.chatgpt.com/docs/app-server#steer-an-active-turn)
- [Ollama chat requests](https://docs.ollama.com/api/chat)
- [Devin hook configuration and output](https://docs.devin.ai/cli/extensibility/hooks/overview)
- [Devin lifecycle hooks](https://docs.devin.ai/cli/extensibility/hooks/lifecycle-hooks)

ACP v2's revised prompt lifecycle remains a draft. This change does not migrate Dinah's ACP v1 client or assume that Devin accepts concurrent v1 prompt requests.

## Delivery failures and recovery

Paul approved closing the recovery and delivery gaps on 24 October 2026. Before dispatching a steer, the host saves its original text, document reference, attachment bytes and prepared content blocks in the bus database. Delivery receipts update that record. A pending record survives a lost response or application restart without authorizing an automatic resend. Queued fallback records remain pending while their later prompt runs, so a crash between starting that prompt and receiving its result cannot erase the recovery evidence.

Opening an idle chat shows unresolved submissions with Copy to Draft and Dismiss controls. Copying appends to an existing draft and preserves attachments. It does not send. A document-context conflict requires clearing the existing selection first. A definite local rejection restores an empty composer; if the person has already typed another draft, the failed submission remains available through Copy to Draft. An uncertain native response stays visibly unconfirmed and requires an explicit copy before another send. Dismissing a recovered submission and deliberately dropping a queued fallback remove it from recovery suggestions.

Ollama propagates conversation-write failures and acknowledges insertion only after the history save succeeds. A failed transaction during insertion may still leave a recoverable file transaction, so it produces an unconfirmed delivery result instead of an automatic fallback. The turn stops on persistence errors. Cancellation immediately closes the adapter's steering boundary. Guidance submitted while an approval is outstanding neither answers the approval nor bypasses its tool result.

## Hook turn ownership

Each hosted hook turn receives a fresh random marker. The bus records the active marker before the prompt starts, and UserPromptSubmit may bind only that active marker. Settlement closes delivery before withdrawing pending messages. Pending guidance carries its turn marker, preventing old rows from leaking into a later turn.

Devin's helper also passes the documented prompt_id through binding and delivery. Events from a previous prompt cannot consume a current prompt's guidance, and an old UserPromptSubmit cannot replace a newer binding. Claude continues using its existing synchronous hosted hooks and session identity. Dinah does not invent a Claude prompt_id or assume that one exists. Its host-turn marker protects binding and delivery windows; it cannot identify an arbitrary delayed Claude event after a newer binding solely from a session ID.

Regression coverage includes restart recovery of attachment payloads, explicit dismissal, delayed Devin events, closed hook turns, cancellation, unresolved persistence without replay, model image capability, and guidance submitted while an approval is unanswered. Browser coverage checks recovery controls, preservation of newer drafts, rejected submissions, uncertain delivery, and attachment restoration.
