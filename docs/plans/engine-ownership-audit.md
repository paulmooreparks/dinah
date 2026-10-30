# Engine ownership audit

Architecture accepted this audit and completed cleanup phase 1 step 7 on 29 October 2026 after reviewing the final reconciliation cut at 28a045d. The SDK migration is outside this work.

## Runtime ownership

| Owner | State and operations |
| --- | --- |
| `sessionhost.Runtime` | Conversation and provider membership, subagent indexes and wake Scheduler decisions share `App.mu` with settings snapshots. |
| `sessionhost.Conversation` | One stable private state mutex protects live identity, activity, steering receipts, replay state and subagent records. Turns retains a separate private submission gate. No second session registry exists. |
| `ProviderLifecycle` | Provider connect serialization, publication, watcher processing, teardown and completion retain their existing order. |
| `SessionLifecycle` and `ClosedSessions` | New/list/open/delete orchestration and tracked close/fallback work have separate owners. `ClosedSessions` owns the single close wait group. |
| `TurnIO` and `Handler` | Turn/steering effects and ACP request/update dispatch use named host dependencies. Permission and file policy remain in the shell. |
| `TranscriptCapture` and `Continuations` | Both use the same `App.cmu`. Conversation history and the process-wide head cache retain the established lock order. Recovery retains its process-wide mutex. |
| `SubagentService` | Child routing, cancellation and event orchestration use the existing Runtime; host callbacks run outside its state lock. |
| `Permissions`, `Journal` and `SteeringLedger` | Permission waiter removal precedes send/close. Journal owns message-time serialization. Steering persistence retains bus transactions without a new mutex or durability promise. |
| `bushost.Service` | The bus handle, watcher and pending questions retain separate watcher-stop and handle-close operations. |
| Applet host, extensions and terminals | Their extracted services retain catalog/maintenance, extension review/ticket/asset and terminal process ownership respectively. |

## Shell responsibilities

App retains native windows, exit coordination, settings and workspace policy, provider preparation/authentication, account presentation, bridge/channel authority, notifications and event transport. The shared question sequence remains on App. Protocol traces and chat logs remain shell effects supplied to session owners. Prompt-history storage and its mutex are separate from captured conversation history.

The host still assembles concrete MCP servers and supplies opaque channel Bind/Revoke capabilities. Provider launch arguments, protected file writes, redaction, settings transactions, saved option policy, path selection and product wording remain named adapters. These adapters do not expose mutable conversation records or provide generic state mutation callbacks.

## Direct wiring

This cut removes 68 internal App forwards after migrating their callers, including test callers. Exported page delegates remain. Callback fields use closures where resolving an owner during construction would initialize another owner. The closures resolve the owner when invoked, retaining the prior lazy evaluation point.

`TestSessionOwnerFactoriesStayLazyAndStable` constructs each of eleven session owners separately, checks stable identity, and verifies that no other session owner or external service was initialized. Constructors assemble dependencies without invoking them. The call-time cycles between session lifecycle, continuation and turn services therefore do not become constructor cycles.

Some remaining adapters perform work beyond forwarding. Examples include `setOptions`, which selects unfinished restoration semantics; journal adapters, which supply shell paths; `askWait`, which translates the internal result; and connection/registry lookup helpers, which hold the shared state lock. After the delivery cut, askWait has only a live-test caller and carries no production orchestration. Native and authentication adapters remain in their existing layer.

## Page and discovery contracts

The exported App signatures in every touched file match the prior signatures. `pageMethods` is unchanged. The page inventory, allowlist and direct bridge tests remain the executable contract; unlisted methods receive no new page access.

Discovery retains the existing extracted source inputs. Shell wiring sources are now explicit additional inputs, including handler, provider/session/closed-session, subagent, continuation, transcript, options and steering adapters. The existing discovery test checks their presence and verifies that mutation changes the digest.

## Shutdown and validation limits

Shutdown still stops the bus watcher, joins tracked session-close and fallback provider-stop work, and only then tears down dependencies. Extension and applet shutdown precede provider shutdown; terminal close, bus departure/handle close, bridge close, logs and notices retain their positions. The join does not promise completion of every turn or bound provider shutdown. Terminal CloseAll still waits for the snapshot's Close calls, without promising every pump or exit callback has finished.

Historical terminal, cancellation, Mermaid, extension focus and ui-state failures remain retained as recorded in the cleanup plan. Passing reruns do not establish their causes. Race-detector validation remains unavailable. This cut does not change browser assertions, SDK behavior, installation or publication.

## Proposed remaining session bus extraction

Architecture approved this boundary on 29 October 2026 after reviewing message 432 and the remaining wake and delivery code. The remaining algorithms will move into one stable `sessionhost.BusChats` coordinator, constructed lazily per App against the existing Runtime and `App.mu`. The coordinator will own orchestration, without adding session membership, a Scheduler, question registration, a watcher, or a bus handle. Conversation state and submission gates retain their current owners.

The following migration will use three separately reviewed commits that extend the same coordinator. Each commit will include moved source in discovery digests and mutation coverage, preserve exported page methods, and run the full Go/browser gates, vet and production build.

| Cut | Engine operations | Shell dependencies that remain |
| --- | --- | --- |
| 1, wake and bus presence | `connectWaking`, `openQueued`, `askToConnect`, `connected`, `closeIdleBackground`, `park`, `unpark`, and the window-change orchestration behind `ChatWindows`; join/leave/title publication sequencing | Saved bus identity allocation/mapping, title presentation, settings persistence, hidden-chat policy, restart guard, clock, event/log transport |
| 2, delivery and questions | `deliverBusRound`, `ringChat`, `busWaiting`, `busAfterTurn`, `askOverBus`, `awaitAskable`, bus prompt assembly and delivered-entry conversion | Authorized directory resolution and closed-chat addressing through `busChat`; agent identity wording, attachment loading/limits, prompt preparation, steering capability policy and event/log transport |
| 3, completion reconciliation | `reconcileChatResults`, `enqueueChatResult`, result-key construction and report serialization | Applet host broker acquisition and a saved caller bus-identity snapshot under App.mu |

The public `ChatWindows` method will remain a page delegate. Internal forwards will disappear after their last caller migrates. Session lifecycle, closed-session cleanup, TurnIO and watcher factories will resolve the coordinator through deferred closures. Its constructor will not call those dependencies or initialize the bus/applet host.

### Named dependencies and state authority

The coordinator will receive the existing Runtime and state-lock identity, a lazy `BusService` accessor returning the existing `bushost.Service`, and lazy accessors for TurnIO and ClosedSessions. It will use named host dependencies for `ChatOfBus`, `AllocateBusIdentity`, `SaveBusIdentity`, `SavedBusIdentity`, `ShownTitle`, `HiddenLocked`, `HoldingTurns`, `Clock`, `EnsureBusChat`, `AgentPerson`, `BusPictures`, `PreparePrompt`, `CanSteer`, `ResultBroker`, `Emit` and `Log`. The final Go signatures will use existing value types or narrow copied target values, without exposing settings or mutable conversation records.

`HiddenLocked` will read settings only while the coordinator holds the shared App.mu. It must not acquire that lock or call back into the coordinator. Saved-identity reads/writes and other shell callbacks that acquire App.mu will run outside coordinator-held App.mu. Runtime lookup, Scheduler decisions, connection completion and the live-background count will remain within their existing transactions. There will be no readiness snapshot followed by a separately locked admission. The existing narrow clock callback retains its invocation branches and position, including invocation within conversation operations; it cannot acquire App locks or re-enter Conversation.

Bus presence sequencing will move alongside park/unpark so that the shell does not retain a second lifecycle algorithm. Joining still changes conversation bus identity before saving settings and publishing title. Leaving still clears conversation identity before ending its bus session. Title publication still updates the conversation's published-title state only after the bus write succeeds. Shell callbacks supply identity and title policy rather than moving those decisions into the engine.

### Wake and presence ordering

Wake processing will close idle background sessions and process tracked closed sessions before checking window readiness and the restart guard. It will then read waking rows, expire admissions, consider each row under App.mu, request reconnects, and drain queued admissions in the current order. Hidden-chat decisions remain in the same admission transaction. Connection completion uses the same runtime state authority and three-slot bound.

Idle candidates are collected under App.mu and parked after unlocking. Parking retains the lifecycle mutation, bus departure and session event order. Unparking retains the lifecycle mutation, registration, join, background timestamp and event order. A registration failure still logs and reaches the existing background/event steps. Window changes still publish the window set before unpark and WindowOpened processing. The clock, idle duration and connection patience remain unchanged.

### Delivery, questions and bus lifetimes

`bushost.Service` retains its current watcher order: labels, result reconciliation, change event, delivery, orphan withdrawal and waking. Watcher-supplied bus pointers and the reported-result map are synchronous borrowed inputs. The coordinator will neither retain a pointer nor capture it in asynchronous work. `NoticeSteers` continues to receive the borrowed delivery check synchronously before each chat is rung.

The coordinator will call the existing service's concrete operations and `With` internally. No arbitrary bus-work callback will be added to the shell adapter contract. `With` currently obtains the handle and invokes its callback; it does not provide a new close lease or hold the service mutex for the operation. This extraction will preserve that behavior and the existing watcher-stop/handle-close separation.

`ringChat` will preserve departure of closed sessions, the restart guard and doorbell check before acquiring the submission gate. ClaimDelivery remains under the gate before capability checks and message delivery. Idle work restoration on empty/error delivery remains where it is. Working delivery submits steering with the same gate handle. Idle delivery emits session and sent events before starting Run, and deferred gate release remains at the same function boundary. Receipt callbacks remain independent of that gate.

`busWaiting` retains the UserPromptSubmit delivery reason and current text/entry result without adding picture handling. `busAfterTurn` retains guard-before-ID ordering and one With callback containing Doorbell, Deliver and prompt assembly. Prompt preparation will not be moved outside that callback. Bus prompt assembly retains formatting, image selection, steering capability filtering, note append, preparation and delivery metadata order. File loading and provider attachment policy remain in the named BusPictures adapter.

`askOverBus` will receive an already authorized target value containing its session ID, workspace and title. It retains sender validation, target address resolution, posting time, PostQuestion, immediate deferred DropQuestion, readiness wait, TakeQuestion, prompt preparation and turn submission order. Post/register and orphan withdrawal remain serialized by bushost's dedicated question mutex. No bus pointer is retained across the readiness wait or provider turn. Readiness and connection-failure checks still precede the cancellation/timer select in `awaitAskable`; the existing poll interval and cancellation selection semantics remain intact. The caller registration/withdrawal regression and original cancellation assertion/deadline remain unchanged.

### Completion reconciliation and evidence

Reconciliation obtains the broker once per round, applies the existing terminal chat-request predicate, and uses the watcher's reported map. Only successful enqueue marks a result reported. Saved owner identity is read under App.mu for each enqueue; missing identity keeps the result available for retry. Report construction preserves raw request/output bytes, errors, workspace-qualified deduplication keys and the rule that historical results without text also omit the answering title. No result is redirected to another chat or workspace.

Coverage will retain compound admission/completion and window-change tests, then exercise idle park/unpark failures and event order; delivery gate contention and failure restoration; synchronous/deferred steering receipts; ask registration, cancellation and withdrawal; result retry/deduplication and legacy report representation. Factory tests will verify stable lazy construction and absence of callback cycles. Shutdown will retain StopWatcher followed immediately by the sole tracked-close join, with handle closure later in its existing position. No broader shutdown guarantee is proposed.

After these cuts, the audit will be checked again against production callers and the page inventory before requesting step 7 completion. Historical failure evidence and the unavailable race-detector result remain open. SDK migration is excluded.

## Wake and presence cut submitted for review

`sessionhost.BusChats` now owns wake admission/completion orchestration, idle parking, window-change handling and bus join/leave/title sequencing. The factory retains the existing Runtime and App.mu and defers external owner resolution. The shell keeps saved identity allocation/persistence, hidden-chat policy, displayed titles, the restart guard and transport adapters. The exported ChatWindows method remains a delegate with its original signature.

The moved bodies retain their transactions and effect order. New owner tests cover HiddenLocked within admission, unlocked host effects, idle departure and failed registration sequencing, and title publication only after a successful write. Existing concurrent admission/completion/window tests call the new owner. Factory coverage includes its stable lazy construction, and discovery tests include source presence and digest mutation. Delivery/questions and reconciliation remain in the shell for the next two separately reviewed cuts; step 7 remains open.

The wake/presence cut passed the full Go suite, both vet gates and the desktop production build. All 94 browser suites passed in 109 seconds at eight workers. Evidence is retained in engine-step7-wake logs and .diagnostics/wake-browser. A mechanical comparison found all 14 moved method bodies unchanged after named dependency/type substitutions, and the touched exported App signatures match their prior forms. These results do not resolve historical intermittent failures or supply race-detector validation.

## Delivery and questions cut submitted for review

The same BusChats coordinator now owns watcher delivery, doorbell admission, user-prompt collection, after-turn collection, bus prompt assembly and question post/wait/take/submission. Authorized directory resolution, agent wording and provider attachment/file policy remain named shell dependencies. The caller converts the authorized target into a value containing only session ID, workspace and title, and the coordinator uses the existing TurnIO owner directly.

Watcher inputs remain synchronous borrowed values. AfterTurn retains one With callback containing doorbell, delivery and prompt preparation. Ring retains its submission gate across work admission, preparation, steering or session/sent events, with deferred release at return. Ask still establishes deferred withdrawal immediately after PostQuestion, and readiness/failure checks precede the cancellation select. The original cancellation deadline and committed-ID registration/withdrawal checks are unchanged.

New tests cover readiness/failure precedence over cancellation, registration before waiting with deferred withdrawal, and preparation failure restoring work before logging and gate release. Existing delivery/attachment/steering/receipt suites use the new coordinator, while the lazy-factory test verifies the added deferred dependencies. Both moved coordinator sources remain discovery inputs with mutation coverage. Reconciliation and the final caller/page audit are still outstanding.

The delivery/questions cut passed the final full Go suite, both vet gates and the production build. All 94 browser suites passed in 118 seconds at eight workers. Initial focused wiring errors and two test-string encoding errors introduced during editing were corrected; their logs remain alongside the passing final logs under engine-step7-delivery-* and .diagnostics/delivery-browser. The original test-string bytes were restored without changing assertions or deadlines. Mechanical comparison matched all eight moved bodies after dependency/value substitutions, and touched exported App signatures remain unchanged. Historical intermittent failures remain open.

## Reconciliation cut and final caller audit

BusChats now owns completion filtering, result serialization and enqueue/report sequencing. Its ResultBroker dependency acquires the existing applet broker once per round. SavedBusIdentity reads the shell settings under App.mu, with no coordinator lock held around that callback. The watcher's reported map and bus argument remain borrowed synchronous inputs. Only successful enqueue marks a result reported; missing identity and failed enqueue remain retryable. Workspace-qualified keys and legacy report JSON retain their prior construction.

A new owner test covers missing identity, cancelled enqueue, successful retry, suppression within a watcher lifetime, deduplication with a fresh reported map, distinct keys for identical invocation IDs in different workspaces, and exact legacy report bytes. Existing async success/failure/cancellation/expiry and text/legacy-reenqueue coverage calls the extracted owner. Discovery includes bus_results.go with mutation coverage. The three moved function bodies match the prior source after dependency/type substitutions.

The final caller inventory separates remaining shell policy and transport from the extracted algorithms as follows.

| Remaining shell surface | Responsibility and engine boundary |
| --- | --- |
| `session`, provider lookup helpers, `beginAsk`/`endAsk` | Shared-lock adapters around the sole Runtime; cycle detection and membership remain Runtime operations. |
| `busCoordinator`, session/provider/turn factories | Stable lazy construction with named dependencies; callbacks resolve other owners only when invoked. |
| `busChat`, `busSessionID`, `saveBusSession`, `withBusServer` | Saved identity, workspace/path validation and concrete MCP assembly; BusChats owns live presence, wake scheduling and delivery. |
| `chatDirectory`, `askTarget`, applet handlers | Workspace visibility, hidden/title presentation, invocation authentication and public input/result formatting; BusChats receives an authorized target value. |
| `RenameSession`, `HideSession`, option/default/substitution methods | User settings policy and saved-name behavior; extracted lifecycle/options owners perform session orchestration. Closed-chat title updates remain part of saved-name policy. |
| Provider adapter preparation and account/file adapters | Executable/configuration selection, authentication, permissions, redaction and attachment policy; ProviderLifecycle and Handler own lifecycle and dispatch. |
| `makePrompt`, `promptFrom`, `busPictures`, `askPrompt` | Editor selection, skills, provider content/file limits and product wording; TurnIO and BusChats own submission and effect order. |
| `flushReplay`, journal paths, logs, emit/notification adapters | Page event packaging, host storage-path policy and transport; transcript capture, Journal and Permissions retain serialization and waiter ownership. |
| Continuation store/config adapters | State-directory selection, saved applet filtering and shell settings transactions; Continuations owns guards, rollback, handoff and recovery orchestration. |
| `askWait` and session/prompt test conveniences | Test compatibility/construction helpers; askWait is referenced only by the opt-in live-test helper after production asks migrated. |
| Exported page delegates | Existing page contract, including workspace/default selection before calling the appropriate owner. |

The audit compared all 271 exported App method signatures against phase 1 step 6 at 28ab8a2, ignoring formatting whitespace; all match. The page inventory and restricted bridge policy are byte-equivalent after line-ending normalization to that baseline. Page inventory/permission tests, discovery mutation checks and the lazy-factory suite provide executable coverage alongside the source comparison. No page method or new agent action is introduced.

Runtime remains the sole membership/Scheduler owner under App.mu. Conversation owns its private state mutex; Turns retains the separate submission gate. App.cmu still serializes transcript work, recovery still uses its process-wide mutex, and bushost still owns the handle, watcher, pending questions and reported cache. ClosedSessions retains the only tracked-close wait group. Shutdown still stops the watcher before joining tracked session-close work, then tears down dependencies in the recorded order. None of these cuts adds a close lease, cancellation guarantee or join of all turns.

Architecture accepted this inventory for step 7 completion. Independent focused result, question, factory, page/bridge, shutdown and digest tests passed on the combined tree (desktop 6.917 seconds), as did vet and the production build. Architecture independently compared all 271 exported App signatures with the step 6 baseline and found no differences; page_api.go is unchanged. Historical failures and unavailable race validation remain open, and SDK migration remains outside this extraction scope.

The reconciliation cut passed the full Go suite, both vet gates and the production build. All 94 browser suites passed in 121 seconds at eight workers. Focused result/async/factory/digest/page tests also passed. Evidence is retained under engine-step7-results-* and .diagnostics/results-browser; no historical failure evidence was removed.
