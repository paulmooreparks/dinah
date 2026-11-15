# Per-window task host binding

This contract lets a registered service carry reviewed background work through the window's existing chat, permission, action and group machinery. It supplies the runtime connection that a static named service does not currently receive. It does not define a task language, scheduler, editor or storage format for a consuming feature.

Dinah Team Lead approved this contract on 15 November 2026. Its baseline is `ff236bd1f514a3739844235cd493478ecc0b12a9`. The Architectural Principles remain at `C:\Users\paul\OneDrive\Documents\Architectural Principles.md`. Implementation and acceptance status are recorded separately in `docs/review/task-host-receiving-2026-11-15.md`.

## Registration and lifetime

Add an optional per-window service factory beside the existing static `services.Register` path. Registration uses `services.RegisterWindow(name, factory)`, where a factory receives a service-scoped `taskhost.Client` and returns a `services.Service` plus a shutdown participant. Existing static services keep their behavior. A duplicate name across either registration path is an error.

The shutdown participant may also implement `services.TaskRecords`, supplying task-generation lookup and a record-ownership lock. Those are private Go ports. The page service is a separate value and must not expose the record-lock methods through reflected page calls. A service without that durable task-store port cannot admit execution.

The window constructs each registered instance once after its local stores and runtime are ready and before serving calls. Registration and construction must perform no native discovery, provider start, session load, prompt or action invocation. A non-window host does not create a pretend runtime; it reports the binding unavailable. Pure source/library services may continue to use static registration independently.

The binding is an interface over the current window's services. It exposes no App, Runtime, native connection, broker pointer, shell channel token or caller-supplied callback that can bypass admission. It becomes unusable after shutdown begins. Services must not cache it across windows or persist it in their records.

The interface carries a window root context and typed ports for task admission, chats, actions, human questions and rooms. The following operation names and result semantics form the contract; method signatures can be arranged into these ports without changing their meaning. A call that only observes local state never creates a provider or broker as a side effect.

## Task identity and authority

The host identifies a task by `(registered service, workspace ID, task ID)`. IDs are opaque; a title or filesystem path does not establish identity. The consuming service owns its task record and supplies a durable record generation. The host separately owns operation receipts and revocable authority. Task identity never implies ownership of documents, attachments, remote records or every chat it mentions.

`Inspect` returns local availability, the current workspace binding, repair/refusal reasons and the legal operations. `Review` prepares a server-held candidate containing the task record generation, declared programs, referenced chats, intended native work, action implementation digests, room changes and deadlines. Preparing a candidate performs no work. The service must include any direct local command in its own review; this binding does not execute arbitrary commands for it.

`Begin` accepts only an unexpired review receipt confirmed through the existing trusted page/person consent path, or authority derived from an existing invocation whose grants permit the requested work. A body field saying that the caller is the person is insufficient. The host obtains caller identity from its trusted call context and validates the receipt against the current task generation and authority state. A bare service request, registered Go object or action name grants no execution rights.

An accepted task receives an opaque, in-memory task handle. Its authority is limited to the captured workspace, operation kinds, programs, chat references, action digests, room changes and expiry. A child task can only attenuate that authority. Code cannot serialize a handle and obtain authority by replaying it after restart. The host retains only the non-secret task and receipt identities needed for audit and recovery.

Workspace resolution uses the saved opaque ID and current authoritative mount/path binding. Every operation rechecks admission. It refuses a missing, changed, ambiguous or inaccessible binding before creating a process or sending work. A service cannot select another workspace by changing a path string. A remount or changed binding requires a new review, rather than silently sending an approved task to a new folder.

Task handles are revoked when their owning invocation/chat authority is revoked, the task is cancelled, the deadline expires or the window shuts down. Person-owned tasks have a host-owned task authority and do not borrow a convenient chat's lease. Chat-owned tasks remain subject to that chat's lifecycle and inherited grants. No task is promoted to person authority when its chat disappears.

## Durable operation identities

Each mutating operation supplies an opaque `operationID`, unique within the task, and its expected task generation. The host derives and persists an intent containing the operation kind, canonical targets, reviewed digest and request fingerprint before contacting a native program or action service. Reusing the same ID and same fingerprint returns the existing receipt or status. Reusing it for different work is refused.

An operation receipt has a typed state: `prepared`, `in_progress`, `succeeded`, `failed`, `cancelled` or `unknown`. It contains the task/operation IDs, generation, target references, timestamps, result identity when known, and a reason. `failed` means failure is established. `unknown` means a request may have taken effect and the available documented interface cannot establish its outcome. Neither state permits silently creating a replacement operation.

Persist a returned native identity at the first documented success boundary, before later setup, title/model changes or UI publication. Carry that identity through subsequent errors. Failure to persist a receipt stops further work and returns an indeterminate outcome; it never claims that allocation did not happen. This binding must reuse the host's allocation/continuation journal when available, with one receipt owner, rather than invent a second allocation authority.

Recovery requires explicit `Rebind` of the task to its durable identity after the consumer has acquired its record ownership lock. The host validates current workspace, task generation, grants and target lifecycle. Rebind returns fresh revocable authority for permitted recovery operations; it does not execute the interrupted task. A live holder elsewhere is refused. Corrupt or unsupported receipts block dependent recovery visibly.

Rebind's recovery authority permits receipt inspection and reconciliation only. It cannot open a chat, take a turn, post a message or invoke a new action. Renewed execution requires a newly confirmed review through Begin, or a current invocation grant checked as Begin checks it. Possession of the task record lock and an old identity never supplies that execution authority.

Native retry is allowed only when the documented native operation is idempotent with the preserved identity, or a documented status/receipt establishes that it did not execute. A lost prompt response does not authorize resending the prompt. An unknown allocation or room post remains unknown until resolved by supported evidence or a person. Titles, timestamps and guessed native IDs are insufficient evidence.

## Chat port

| Operation | Contract |
| --- | --- |
| `DescribeChats` | It reads local canonical identities, current heads, workspace, program and lifecycle/refusal information without discovery or loading. |
| `CheckProgram` | It evaluates current configuration and policy locally. It does not substitute another program or run a catalog probe. Native verification belongs to explicit work admission. |
| `OpenChat` | It opens a reviewed existing chat through the normal resolver and lifecycle, or allocates a reviewed fresh chat with a durable allocation receipt. It returns the current canonical chat/native identity and normal option state. |
| `TakeTurn` | It obtains the ordinary exclusive turn admission, sends one labelled turn through the normal queue, file authority and permission machinery, and reports completion, refusal, cancellation or unknown outcome. |
| `ReadOptions` | It returns an already opened chat's supported options. It starts no unrelated program. |
| `UseModel` | It applies only a documented offered model option while the task holds exclusive turn/configuration admission. It returns a conditional restoration receipt. |
| `CancelTurn` | It asks the current owner to cancel the matching admitted turn and waits for bounded settlement. It does not cancel another turn that later occupies the chat. |
| `ReleaseChat` | It releases the task's runtime interest through the documented close path when safe. It never deletes or archives provider history. |

All existing working, pinned, provider policy, file-access and continuation checks continue to apply where relevant. A task cannot take a turn in a working chat or independently change a person's concurrent model selection. A model restoration applies only if the task still owns the configuration revision it changed. If that revision changed elsewhere, restoration is refused with the current state; it must not overwrite the newer choice.

The model-change receipt records the original and requested values before dispatch, then records whether the host established application and its resulting runtime configuration revision. All ordinary option setters share the same serialization boundary and advance that revision, including a person selecting the same value again. An intervening change away and back therefore invalidates restoration. A reconstructed runtime has a new revision identity, so an old receipt cannot authorize restoration after restart. The task host accepts a restoration reference only to its own recorded model-change operation for the same chat. Caller-supplied values or revision strings cannot invent a restoration receipt.

Each permit carries the reviewed JSON input. Begin retains the host's stored copy, and Start checks the input fingerprint as well as operation kind, target, program and action digest before the first dispatch. Changing input under a new operation ID cannot enlarge the reviewed authority. A review question includes operation summaries and input fingerprints without exposing raw operation inputs in question events. The consumer's task page remains responsible for presenting the task definition for review.

Fresh-chat failure preserves known native receipts or unknown allocation state. It must not call a native delete as rollback. Successful named chats are recorded through the normal host identity path. Runtime release cannot erase a chat record, and marking a chat as task-related must not let it escape the current logical-chat resolver or Recycle Bin rules.

`OpenChat` and `TakeTurn` recheck canonical ownership and lifecycle immediately before admission. An ownership ambiguity, live allocation reservation or any nonempty removal reservation refuses work. A chat entering the Recycle Bin revokes every old lease and pending authorized operation. The binding cannot restore it, wake it, queue a result to it or reopen its provider session on behalf of an old task. Restore does not revive the old handle. A later explicit task rebind/open must obtain fresh authority and leave retained deliveries dormant until their normal policy permits them.

The provider may be shared with unrelated chats. Releasing a task must not stop their provider process. If the program cannot unload one session safely, use the existing disclosure and deferred-release behavior. Never manufacture a successful unload or stop the shared process to satisfy a task's cleanup.

The desktop implementation retains a private runtime interest for each successful OpenChat operation. That interest prevents the normal closed-session watcher from ending the conversation. ReleaseChat accepts only that task's recorded OpenChat operation, releases its interest once and reports that native unloading remains deferred to the existing close policy. Cancellation and window shutdown release the same interests. They do not remove the conversation or provider history.

## Action port and human questions

`DescribeAction` returns the current action schema, replay class, capabilities and implementation digest through the existing registry. `InvokeAction` uses the task/operation identity, captured workspace, owner, reviewed digest, input fingerprint and deadline. The broker remains the sole owner of invocation persistence, replay policy, consent, cancellation and artifacts. The task journal stores its invocation reference, rather than duplicating the broker's result as a second execution authority.

Changing the action implementation digest or relevant input after review refuses invocation. A recovered once-only action is queried by its saved invocation ID; it is never reinvoked under a new ID because a service call timed out. Pure actions may follow the broker's documented replay policy. An irrecoverable once-only outcome returns `unknown` or the broker's established failure and a repair reason. It does not fall back to a different tool.

`CancelAction` revokes and cancels the matching owner across retained broker generations without creating a new broker just to cancel it. Cancellation acknowledges a request to stop; it does not claim rollback of a write already completed. Final receipt state comes from the broker. A document or file an action returned remains governed by its own store; cancelling or deleting the task does not delete it.

`AskPerson` creates a host-owned request with task ID, asking sequence and allowed answer schema. Answers come through the existing trusted person-response path. Old sequences, answers from agents and repeated conflicting answers are refused. A task can show a link to the request but cannot answer it itself. Cancelling or revoking a task retires pending questions and rejects late answers. The consuming feature remains responsible for interpreting an accepted answer in its own state machine.

Task questions appear in both Decisions and Notifications. The two surfaces share the host-generated key and the existing person `Answer(key, optionID)` path. A task question shows the registered service's display name, the consumer's task title and the saved workspace's display name. It links to the task's own page and never offers Open Chat, creates a chat-cache entry or appends a transcript answer row. Answer, cancellation and revocation remove the pending question from both surfaces.

The `task:permission` event and `PendingTaskPermissions()` query return identical records with `key`, `service`, `serviceName`, `workspace`, `workspaceName`, `task`, `generation`, `taskTitle`, `question`, `options`, `askedAt` and `target`. `options` retains the existing `optionId`, `name` and `kind` fields. `askedAt` is the stable host creation timestamp. There is no session ID, authority handle, review token or raw operation input. The corresponding `task:answered` event contains `key`, `service`, `workspace`, `task` and `generation`; it indicates that the question ended without implying that the person answered it.

The registered consumer supplies task title and page destination through a private metadata port. The registered service supplies its display name, and the host resolves the saved workspace name. Caller JSON and opaque identifiers cannot supply display names or a URL recipe. The host validates a destination against the service's declared page ownership and builds the existing addressable page-state query. The target representation is `{ "href": "..." }` when available, or `{ "reason": "..." }` when unavailable, with exactly one field present. An unavailable task page does not prevent the person from answering the question. A destination cannot select a chat, a remote URL, another service's page or a different workspace.

Pending task and chat snapshots reconcile independently. A terminal event wins over an older pending snapshot, including an event received while the query was in flight. Ended question keys stay suppressed for that page lifetime and are never reused by the host. Both answer controls share the same pending state, so one submission disables both surfaces until the authoritative result or reconciliation arrives.

## Room port

Room support is a separately advertised capability, initially absent unless the complete adapter below is implemented. Ordinary group/team support alone must not advertise it. A consumer whose reviewed work requires rooms refuses the entire start before any step, prompt or action if this capability is absent.

`DescribeRooms` resolves local chat and group/team identities in the captured workspace, with current eligibility and legal changes. It does not find members by prompting programs or load a closed chat. Ambiguous title lookup is returned as a choice/refusal rather than selecting a candidate. Canonical identities and ownership generations are captured in the review.

`EnsureRoom` performs only the reviewed create/add-member changes through existing checked operations. It preserves retained group identity and per-member partial outcomes; retry cannot silently create another group. It never replaces an existing team's lead under an operation that only converts an ordinary group, violates immediate-lead membership rules or silently adds unreviewed members.

`ClaimRoom` durably gives one task phase exclusive ownership, including task generation, phase ID, eligible lead, accepted exit labels and recorded root message IDs. Competing claims report the holder and perform no post. Claims survive restart as recovery records but carry no reusable runtime authority. A different phase cannot adopt a claim by guessing its key.

`PostRoom` records a stable operation/request ID before addressing the reviewed lead through the normal messaging path. It returns the persisted root message ID. Repetition returns that same root; response loss must not create a duplicate post. Recycled, ambiguous or otherwise inadmissible members remain refused. Delivery continues to use the host's normal permissions and admission.

`ObserveRoom` reports ordered lead messages after a cursor with their root correlation and delivery activity. Only a message from the captured current lead, correlated to a recorded phase root and matching an allowed exit, can finish that phase. A quiet room is a waiting state, not a successful completion. Unrelated messages and late replies to another phase cannot end it. A cancelled observer unsubscribes without polling indefinitely.

`ReleaseRoom` releases only the calling task's claim and observation. It does not remove the room, its members, messages or native chats. `RecoverRoom` validates the claim and existing roots without posting again. If the current host cannot supply durable root correlation, idempotent posting and exclusive claims, it reports room support unavailable. It must not approximate the protocol with title matching or an uncorrelated last line.

## Cancellation and shutdown

Task cancellation first closes admission and revokes its leases, then cancels queued work, current owned turns, action invocations, room observations and human questions. The host and consumer both persist cancellation progress. Completion racing cancellation is recorded as the actual established outcome, including completed writes. Cancellation cannot erase a receipt or return another task's shared resource to an earlier state.

The per-window participant registers owned background work with the existing exit guard before launching it. Closing its page or completing the two-minute service request does not implicitly cancel the durable task. The task's explicit cancel, authority expiry or window shutdown governs its lifetime. The service cannot detach an untracked goroutine that outlives shutdown.

Shutdown proceeds in this order.

1. The host closes new task and operation admission and revokes handles.
2. Each participant checkpoints its task state, requests cancellation and joins its workers within the host's shutdown budget.
3. The host settles outstanding operation receipts where possible, cancels questions and observations, and releases documented runtime interests and room claims that it can establish as safe to release.
4. The host persists interrupted or unknown outcomes for unsettled work before its existing broker/provider teardown completes. It does not claim success merely because a deadline expired.

The shared teardown must not stop the provider or broker before admitted cancellation and receipt checkpoints have had their bounded chance to finish. Failure to persist is surfaced in the shutdown result/log and blocks automatic dependent replay on the next start. No fixed sleep is a correctness condition. A service factory/participant failure is explicit; it cannot leave partially admitted work running without an owner.

Closing admission revokes all consumer execution handles. The host retains a private cleanup path for cancelling operations already admitted, joining participants and saving their receipts. This path cannot admit a new operation, restore authority or send a fresh turn. It remains usable through checkpoint completion and closes before provider/broker teardown. Tests must revoke execution at shutdown and then observe cancellation and durable completion through this separate path.

## Errors and observable behavior

Operation results identify their receipt state and a stable refusal code with explanatory text. Required codes include `unavailable`, `not_authorized`, `workspace_changed`, `stale`, `busy`, `removed`, `ambiguous`, `unsupported`, `held_elsewhere`, `cancelled`, `deadline_exceeded`, `unknown_outcome` and `persistence_failed`. A timeout describes the bound that expired and does not by itself classify an external effect as failed.

Representations expose only legal actions, each bound to the current identity/generation. A previously displayed command is still checked at use. Observation events carry task ID, generation and a monotonic per-task sequence so a consumer can discard stale events without reordering state. Reconnect uses local status/receipt queries and cannot restart work. Events and logs exclude raw authority handles, channel tokens, credentials and private tool parameters.

The binding does not grant final native deletion, direct writes into a program's private files, archive-as-delete substitution, arbitrary workspace access or document approval authority. Those operations retain their separate existing contracts. A required external capability that lacks a documented interface is unavailable; measurement of undocumented behavior cannot enable it.

## Review and acceptance gates

The implementation must have injectable lifecycle, broker, clock and room peers, with counts observed at the real host entry points and persisted receipts read without test-side filtering. Required acceptance includes the following cases.

- Constructing factories, inspecting tasks/chats, listing records and reloading pages starts no program, loads no session and invokes no action.
- A positive control opens a permitted chat and takes one counted turn through the existing permission and file-authority paths.
- A changed workspace binding, stale task generation, foreign owner, forged consent, absent capability, working chat or removal reservation refuses before external work.
- Allocation receipts survive setup failures; a lost response produces unknown outcome and never an invented native ID or duplicate automatic allocation.
- A repeated action operation recovers the same invocation; changed fingerprints/digests refuse; cancellation racing a completed write preserves the real result.
- Binning a referenced or owning chat revokes old authority and cancels its pending work; addressed work refuses while binned, and Restore never revives an old token or causes a provider load.
- Two tasks cannot overlap a turn or overwrite a person's newer model choice; releasing one leaves unrelated chats and their shared program running.
- Shutdown joins counted owned work within a fake-clock-controlled budget and persists interrupted/unknown states; reconstruction observes records without launching or replaying work.
- Unsupported rooms refuse at review. Supported rooms preserve partial creation identity, exclusive claims, idempotent root posts and correlated lead completion across restart, while unrelated or late messages and quiet state cannot finish a phase.
- Human requests reject stale sequences, agent answers and post-cancellation answers. Task cancellation and final chat deletion leave linked documents, drafts and remote records unchanged unless an independently authorized action actually changed them.

No acceptance pass is claimed by this document. Consumers that depend on execution remain gated on the implementation checks and independent acceptance. Pure authoring and storage surfaces can proceed independently of this binding.
