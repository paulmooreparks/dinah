# Bus attention and team communication

Paul authorized this work on 31 October 2026. Architecture owns the contract and implementation. The existing UI and Release chats provide independent verification and deployment. The current chat-loading correction completes first. After the bus correction is installed and Paul restarts, the standing team can be renamed and reconfigured. The deferred taskbar label and chip/glyph work remain in the queue.

## First release

Group visibility and requests for action must be distinct. Every group member can read the group history. Informational deliveries must not start an agent turn, reopen a closed chat, steer a working chat or consume a group turn allowance. Pending notices may accompany the recipient's next independently initiated turn. Explicit requests retain reliable queued delivery and existing trust, membership, retirement and budget checks.

The tool contract must identify action recipients separately from informational recipients. Existing `mentions` callers currently request action, so compatibility must preserve that behavior rather than silently making existing requests passive. The routing decision below defines the fields and persistence. Invalid or conflicting routing must fail visibly. Human notification and the existing member-to-lead relay must remain explicit and must not be inferred from text.

Direct human requests continue to wake their intended chat. A person's unaddressed message in a team group continues to ask its lead, following the existing team contract. Agent informational group posts remain passive. A working agent receives an action request through its supported delivery mechanism; passive notices never cause steering. Delivery receipts remain receipts, without implying acceptance or completion.

Passive deliveries remain readable and are not marked consumed merely because a wake query skipped them. Restart and reconnect must preserve pending action requests and informational delivery state. Existing closed-chat and retired-member exclusions remain in effect. Delivery changes must be enforced in the bus and session scheduling paths, rather than relying on an agent to ignore a turn after it has started.

## Routing compatibility decision

For group sends and replies, `to` (or the original message's group) defines the audience. A new optional `actionRecipients` array names the members expected to act. Omission or an empty array makes an agent's group post informational. The existing `mentions` array remains a compatibility alias for action recipients; providing both fields is rejected, including explicit empty arrays. Group membership, `everyone`, human notification and team relay resolution use the existing validated recipient resolver. This avoids creating a second identity-resolution path.

The stored group `mentions` field already contains resolved action identities. The first release retains that storage format and changes no historical message meaning. Display and prompt wording will describe those identities as action recipients. A group's audience remains its membership; no redundant informational-recipient list is needed. Direct sends retain their current request semantics, and supplying group action routing on a direct send is rejected. Existing human composer mentions retain their action semantics, with explanatory labels updated to describe passive delivery to other members.

The bus doorbell and Stop continuation query must count action deliveries only. Ordinary turn-start delivery may include pending notices, which do not spend the group's action-turn budget. Existing ask/completion and human relay contracts remain unchanged. Native hook delivery points must be inspected so that a passive notice cannot create a turn indirectly through a tool-completion or Stop hook.

## Shared prompt wording

The shared communication prompt and tool descriptions must agree with the implemented routing. Agents send when assigning work, requesting a decision, reporting a blocker, changing a shared contract or handing over a reviewable result. Action recipients are only those expected to act. Agents do not acknowledge routine updates or repeat accepted plans. Handoffs identify the result, exact artifact, verification, limitations and requested action, with the durable task record updated before handoff. Ownership stays with the sender until accepted.

The lead reports to Paul when a decision is needed, a usable result is ready or a material problem changes expectations. Routine internal traffic does not require a user-facing summary. Role ownership and authorization rules remain separate from these communication instructions.

## Verification

Tests must cover idle, busy, closed and retired recipients; group and direct requests; passive posts; human routing and lead relays; budgets; trust boundaries; restart recovery; and compatibility with existing callers. Verify that passive-only traffic produces no turn or provider call and that a later genuine turn receives the pending notice once. Verify that explicit requests still arrive and that duplicate delivery protection remains intact.

Record notice-only wakes and delivered action counts using existing diagnostics where possible. Do not label an empty model answer as a no-action turn without a defined measurement. UI review must verify any routing controls and labels. Release runs the complete required gates on the merged candidate and reports installed and running versions separately.

## Follow-up scope

Request lifecycle tracking and queued shared-resource leases are separate changes. The 31 October test-slot conflict is an acceptance scenario for leases. No model-based per-message routing judge is required.

The installed applet catalogue already exposes `dinah.team/start@1`, `assign@1`, `list@1` and `retire@1`. Start accepts title, brief, workspace, provider and model, with provider-default permissions rather than inherited authority. The inspected Chat and Team action catalogues do not expose rename, reparenting an existing chat or reasoning-effort configuration. Those are potential team-management additions, outside this first bus release. Any addition must validate provider-supported settings, preserve chat history and identities, enforce ownership, and expose its effects to the person.

## Implementation and evidence

The candidate changes the shared wake predicate to action requests only. Doorbell, arrival previews and Stop use that predicate. Doorbell delivery rechecks eligibility inside its transaction before consuming anything, so a request claimed by another consumer or a spent group budget cannot leave a notice-only provider turn. Existing native PostToolUse hooks can collect passive context during an already active turn; they cannot create one. Window delivery checks both idle and busy paths and AfterTurn.

The MCP contract adds actionRecipients to send and reply. Both fields supplied together, including two empty arrays, fail. Explicit action routing on a direct message fails even when empty. Legacy mentions and direct sends retain their meaning. Persisted messages and group membership remain unchanged. No provider, team configuration or installation is changed by this candidate.

Focused package checks cover bus, MCP server, session host and shared communication. New tests exercise both API spellings, conflicts, group replies, direct-send rejection, idle and busy no-prompt behavior, passive restart recovery, stale doorbells and one-time subsequent delivery. Existing tests retain retirement, human notification, team relay, trust and budget coverage. Tests that formerly required notice-only wakes now require passive delivery instead. The old direct-message fixtures that forced DoorbellTurn despite exceeding hop limits now use an independently initiated delivery; the transactional guard enforces the scheduler's existing hop limit.

The initial browser attempt could not load playwright-core in the new worktree. After npm ci using the committed lockfile, group-notice-note, composer-completion and group-chat-composer passed in 20 seconds with two workers. The generated group-notice-explanation screenshot was inspected and its disclosure is readable without clipping. Browser failure records are under .diagnostics/bus-attention-browser; passing failure records are under .diagnostics/bus-attention-browser-ready. Because that run used a relative output path, the screenshots are under cmd/dinah-desktop/frontend/e2e/.diagnostics/bus-attention-browser-ready.

No production communication-overhead metric is claimed. Existing delivery records and prompt-preparation assertions provide evidence for this change; aggregate task-efficiency measurement remains follow-up work. Independent review and the full merged release gates are required before installation.

The final focused gate passed bus (18.937 s), MCP server (2.499 s), session host (5.352 s), communication (0.303 s) and desktop (133.492 s) through scripts/go-gate.ps1 with count=1 and a three-minute package timeout. go vet passed for those five packages. Whitespace checks passed. Earlier development runs exposed obsolete wake expectations and were corrected against the new contract; one earlier invocation was stopped and is not a completed gate.

The candidate is ready for independent code and affected UI review. Architecture retains implementation ownership until that handoff is accepted. Release remains responsible for the complete merged gate, installation and version identity. The team restructure and deferred workspace-first taskbar label remain untouched.
