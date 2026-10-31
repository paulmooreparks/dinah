# Codex persistence before publishing a new conversation

Engine prepared this contract on 2026-11-01 from released commit `40d133aeb8e2d4181d75aa89c0f225ef275f43fc`. Product implementation and independent contract acceptance are pending. The shared test slot is released after the two isolated probes described below.

## Incident and evidence

The four reported PUDL Codex failures have saved Dinah history and pending continuation handoffs, but their preserved native message collections are empty. Their native IDs are absent from the coherent provider database snapshot and active/archived rollout filename inventory. Logs independently report reopening errors for Team Lead, Verification, Build and Release, and Implementation Engineer. Inherited weekly-limit errors belong to prior history; they do not prove a first native turn occurred in these four sessions.

The applet inventory resolves the current PUDL team to the explicitly recovered Team Lead and three failed Codex members. A separate closed Claude Build and Release chat also exists. Its matching title does not identify it as the reported Codex failure. Eleven of 44 saved Codex IDs lack native database rows; that inventory includes the four reported failures and superseded or otherwise unverified records. Absence alone does not establish eleven failures.

Engine preserved settings, conversation snapshots, logs, SQLite backups and applet lookup receipts under `.diagnostics/`. No existing chat was reopened or mutated. The originals remain available for explicit recovery. The evidence manifest records file hashes without publishing private transcript contents.

## Reproduction

Both probes used Codex CLI `0.162.0`, separate fresh `CODEX_HOME` directories and empty fixture workspaces. The exact scripts, requests, responses, process identities and exit receipts are retained. The probes supplied no credentials and sent no `turn/start`. Provider startup attempted unauthenticated websocket prewarming and received HTTP 401; there was no model turn. All four owned app-server processes exited normally after stdin EOF.

Each probe started two non-ephemeral threads. The control thread was readable in its original app-server process, but after process restart its original ID returned `thread not loaded` from `thread/read` and `no rollout found` from `thread/resume`. The other thread received a nonempty `thread/inject_items` request and remained readable and resumable with the same ID after restart. The second probe used a developer-role application-context message and explicitly unsubscribed both threads before stopping the first process. Neither probe depended on an undocumented delay or file-creation check to choose its protocol actions.

The observed mechanism matches the original records: Dinah can publish a new Codex ID and continuation handoff before a model-visible history write has made that thread durable. These probes establish a reproducible mechanism. They do not reconstruct every event of the four original sessions.

## Supported persistence boundary

The official [Codex App Server documentation](https://developers.openai.com/codex/app-server/#inject-items-into-a-thread), read on 2026-11-01, states that `thread/inject_items` appends Responses API items without beginning a user turn and persists those items in the rollout. The installed CLI schema exposes this operation without experimental API opt-in. Its thread path field is explicitly unstable, so path existence will never govern production behavior.

The proposed adapter contract requires a successful application-context write before returning a newly created session to the host:

1. The adapter starts a non-ephemeral thread using its existing workspace, approval and managed-server configuration.
2. The adapter validates the returned native ID and injects one developer-role message containing truthful host context: `This conversation is hosted by <application name> in workspace <quoted workspace>. This application context record is not a user request.`
3. Only after that request succeeds does the adapter register the session and return its ID through `session/new`.

The context is real integration metadata. It contains no fabricated user request, continuation transcript, secrets or applet authorization. It starts no model turn and does not consume the pending handoff. Its text becomes model-visible under the documented API contract, which is an intentional part of this proposal requiring review.

A rejected, unsupported or interrupted write fails session creation. The adapter attempts bounded best-effort unsubscribe of its own newly allocated thread and reports the original failure. It does not publish the ID, select another persistence mechanism or replace an existing identity. A cancelled request can leave an unclaimed provider thread if the provider completed the write without returning its receipt; the host still receives no successful new session.

The host already creates the provider session before committing continuation history and transferring bus identity. Keeping the new publication boundary inside the adapter preserves that ordering for fresh chats, provider switches, ordinary continuations and explicit saved-history recovery. Existing saved histories and routing remain intact if creation fails. Resume continues to validate the original workspace and native ID and does not inject another context message.

Existing missing native IDs remain explicit recovery cases. This correction does not recreate their IDs, silently start successor conversations, rewrite provider storage or change the saved-history recovery confirmation.

## Required regression coverage

- A newly created session must receive its context write before publication, without a user turn.
- The same identity must load after unsubscribe and simulated provider restart, including when no initial turn occurred or a later initial turn failed.
- A rejected or unsupported context write must fail creation, leave no registered adapter session and preserve the parent handoff and bus identity.
- Resume must retain workspace validation and must not inject duplicate context or change the native ID.
- Explicit saved-history recovery must retain ancestry, pending handoff, title and team routing through the new creation boundary.

The immutable probe contract is handed to Team Lead and Quality before implementation. Engine retains implementation ownership. Quality's frontend browser verification follows this explicit slot release. No full gate, product correction, installation or running-app restart is claimed here.
