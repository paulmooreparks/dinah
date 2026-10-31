# Recurring chat loading investigation

This investigation follows the Judgements popup correction. The acceptance contract and queue remain in Architecture's `docs/plans/recurring-chat-faults.md`. The original false-deleted group incident, saved-restore timeout and taskbar click failures remain open. No saved conversation is rewritten and the running application is not restarted for this investigation.

## Preserved observations

The earlier screenshot shows a blank foreground window titled Chat and a populated background window also titled Chat. The later accessibility capture identifies a Codex Engine Cleanup window displaying 89,040 history parts. These observations occurred at different times and do not establish that the blank foreground window was the old Claude session.

Engine preserved the affected Codex saved file at 13:00 on 31 October 2026. Its 35,500,482 bytes contain 115,660 parent-history entries and 4,611 current entries. The file is incomplete and links to the former Claude session. The snapshot SHA-256 is `e29204cfaf65650a78c988725fb33dea25fa08bd6af99b0162862efae4233eaf`. Engine's copies and count report are in `.worktrees/engine/.diagnostics/recurring-chat-20261031/`. These are saved entries; provider replay events and rendered parts remain unmeasured. Crash evidence remains in Engine's earlier switch diagnostics and does not establish a cause.

The checkpoint fix `85b22f216289844ead2977f4871a785dbc36741e`, merged through `bda2ab1`, is an ancestor of the incident's reported installed revision `cbd5766`. The installed executable had `vcs.modified=true`, and the screenshot's loaded executable identity was not independently established. The fix's source remains in the current lifecycle code. Its background-member regression holds journal catch-up after provider opening, without delaying the preceding continuation assembly or asserting title/member presentation throughout that interval. Separate saved-restore tests delay provider opening while a small saved copy is visible.

## Source findings to reproduce

`openResolvedSession` requests `SessionContinuation` and walks every returned history entry in batches of at most sixteen before invoking `OpenSessionAt`. The continuation bridge returns the saved parent history. `prependContinuation` creates rows, and `row()` increments the replay counter while loading unless the drawing-saved flag is set. Consequently the counter can include locally rendered continuation entries before the provider-open request starts. This is distinct from measuring provider replay events. The affected file's large parent history makes this path relevant, but its execution during Paul's incident remains unverified.

A background connect without an existing view skips the initial saved-copy draw. Opening its window while it is already loading reaches restoration guards that skip drawing that copy. This path needs a controlled reproduction with a visible window attached during continuation assembly and before provider opening completes.

Persisted user titles are available through `State().titles`; `SavedConversation` supplies content, completeness and journal position without title/team metadata. `ChatLead` and `Team` can query persisted relationships independently of provider opening. Engine advises against using `SessionsAt` as an immediate metadata query because it can connect providers and list remote sessions. The ordinary session store starts with an empty title and no membership, while window registration supplies the generic Chat title before its applet starts. Persisted window titles exist separately, so restoration must be checked both with and without that cache and while other state queries are delayed.

## Reproduction and ownership

Front End will test the whole interval from window creation through saved-copy loading, continuation assembly, provider opening and journal catch-up. A background connection followed by window opening must show available saved content and known title/membership throughout the unresolved interval. Cold restoration, cross-workspace opening, provider continuation and delayed/failed metadata reads need separate checks. Fixtures will record saved entries, rows constructed and displayed counters independently. Fixture success cannot close the installed report.

Engine owns provider resume/replay, adapter emission and saved-history growth. Its assessment will distinguish returned thread turns/items, live notifications, emitted updates and persisted entries. The original saved files and earlier crash logs remain unchanged. Installed verification must identify the running build and exercise the affected continuation shape after a coordinated restart.

Release freed the shared slot after the Judgements release. Front End then ran the preserved-shape baseline on committed `5c52936`. The debugger connector is unavailable in this session; source inspection and controlled test boundaries will be used without temporary product logging.


## Completed baseline

The isolated reproduction loaded the preserved snapshot with text redacted while retaining its entry roles, identities and counts. The continuation response returned at 1,098.6 ms. `OpenSessionAt` began at 132,046.2 ms, approximately 131 seconds later. The fixture emitted zero provider history events. Before provider opening began, the sampled counter reached 115,648 parts labelled as history sent by Agent; the final pending-opening sample reached 115,660. All 7,244 frames lacked the available saved-content marker. Retained rows peaked at 600, so retained DOM limits did not prevent constructing and discarding the entire parent history. Closing and reopening the window did not cancel its background connection. The known title and independently resolved membership remained stable in this baseline; it does not reproduce their separate reported faults.

The report, frame samples and screenshot are preserved in `.worktrees/front-end/.diagnostics/chat-loading-baseline/`. This establishes a frontend workload and progress-attribution defect in the controlled path. It does not establish which build painted Paul's screenshot, explain the historical saved-file growth, or establish a crash cause. The snapshot's original file remains untouched.

## Approved implementation ordering

Provider opening will start independently of local display construction. A saved snapshot is display evidence while opening is unresolved. Its recent content will be built in a separate bounded display record so provider updates cannot mix into that construction. Closing a window keeps the connection alive; reopening attaches the same display and session rather than starting a second connection. Late display work must check that the session still exists before attaching.

After provider opening returns, the page will read a fresh saved snapshot. It may establish a journal position only when the snapshot's generation matches the current session generation and its checkpoint is valid. The page then displays that snapshot and reads journal entries after its checkpoint. Events arriving between snapshot reads and presentation remain recoverable through the current journal. A pre-open checkpoint is never transferred to a different generation. Missing or mismatched checkpoints require replay/catch-up from the current generation's beginning into a separate live log; the pre-open display remains visible until that replacement is ready.

Ordinary opening will not construct every parent-history row before calling the provider. A fallback without an available saved snapshot will display only the recent continuation window, retaining an accurate omitted-entry marker and a distinct Show Saved Chat History control. No saved file or provider input is truncated by this display policy.

Completeness and warnings remain independent from availability. The preserved incomplete copy is readable while its warning remains visible. Local saved rendering must not increment a counter attributed to provider history. Provider-event progress and retained/constructed display rows will be measured separately.

Known titles and team relationships will survive loading. An unresolved title retains its saved window title until authoritative naming arrives. Unresolved membership will be presented as a lookup in progress, with ordinary-chat controls withheld until the relationship is known, rather than temporarily exposing a member as an ordinary chat. A failed lookup must retain a previously known relationship and disclose the failure. Metadata reads must not wait for provider startup.

Architecture confirmed that no new opening-cancellation action is requested. Stop will be exercised only where the existing working-turn state exposes it, and it will retain its provider-turn cancellation contract. Local continuation-render cancellation is currently unavailable. Closing a window must release its rendered attachment safely while permitting reattachment without duplicate rows or stale updates.


## Saved-history access

The operational Show the Chat Log action reveals the current session log. Engine checked twelve sampled older parent entries against the affected child log and found none. That log also rotates and limits individual blocks, so it cannot provide full continuation access.

Engine commit `2e0c7a8d2d65a08b3f8cbc4c3fa6b685b8ec67d4`, integrated here as `5299fb9`, adds ShowSavedChatHistory. The control is labelled Show Saved Chat History. Its tooltip explains that it creates a full saved-history snapshot and reveals the Markdown file in the file manager. Export or reveal errors use the existing visible failure notice. The operational log command keeps its existing meaning.

The export contract is recorded in `docs/plans/saved-history-export.md`. The snapshot includes saved parent and current entries in order, their origins and boundaries, capture time and incomplete-history warnings. It applies the existing credential redaction without summary, rotation or block truncation and leaves the source unchanged. Engine verified representative oldest, middle and newest entries in a 45,295,952-byte export of the preserved 120,271-entry shape. That establishes export completeness; UI review must still verify the control, file reveal and usable external viewing of older entries.

## Implementation and negative cases

Provider opening and the initial saved read start independently. Saved display construction uses a separate record with a recent 400-entry window. It can finish while its window is closed. Its final attachment removes the temporary working log, preventing duplicate visible rows. A missing snapshot uses the same bounded parent display. The omitted-entry marker reaches the full saved-history export.

After opening, only a fresh snapshot with the current journal generation establishes a checkpoint. A missing through value means zero under the bridge's optional zero-value encoding. Missing or mismatched generations replay the current journal from zero. Journal entries that arrive between the fresh read and presentation are caught up after its checkpoint. An unsuccessful current-history read retains the available copy with a specific failure message; journal presentation waits for retry and a verified current position. Provider-opening failures retain the copy too, with their separate error message.

A persisted title can name the window before its applet mounts. Sparse session updates do not replace a known title with a generic title. Team membership resolves independently of provider opening. Pending membership withholds ordinary-chat controls, and failed refreshes retain a known relationship with a retryable error.

The focused loading matrix holds saved reads, membership reads and provider opening independently. It covers matching, zero, missing, mismatched and changed checkpoints; unavailable snapshots; cached titles; cross-workspace opening; failed provider opening; and failed current-history reads followed by retry. It samples every rendered frame through close/reopen and catch-up, checks content and title/member presentation, and verifies Stop still calls provider-turn cancellation. The export failure case checks the visible error notice. Existing session recovery, saved restoration, sequence, continuation and Stop suites remain part of validation.

The first corrected large-shape reproduction dispatched OpenSessionAt 7.6 ms after background connection began and showed saved content after about 664 ms. These are controlled-fixture request and display measurements. They do not measure provider readiness, real startup or model latency. The original screenshot, saved-history growth, apparent crashes, original false-deleted incident and intermittent restoration/click failures remain open. Installed verification requires the affected continuation shape and a coordinated restart with the running identity captured.


The final preserved-shape rerun in `.diagnostics/chat-loading-final-large/` dispatched the fixture provider-open request at 649.4 ms, 8.1 ms after background connection began. The window attached at 687.3 ms and first displayed the saved-content marker at 1,649.4 ms, about 962 ms after attachment. It retained at most 406 rows, emitted zero fixture provider events and completed close/reopen without a page error. The initial 27 sampled frames preceded saved-content display; they contained the loading presentation. This run still measures controlled request dispatch and local display, with the real provider held unresolved.

Six focused suites passed in `.diagnostics/chat-loading-failure-focused/`. After extending failed-read coverage to sent and turn events, the loading, sequence and Stop suites passed in `.diagnostics/chat-loading-final-focused/`. The earlier nine-suite run in `.diagnostics/chat-loading-frame-focused/` covers workbench, team and restoration consumers. The full candidate gate is recorded separately below when complete.


## Candidate gate corrections

The first full gate passed 155 of 159 suites in 314 seconds. Its failures remain in `.diagnostics/chat-loading-full-final/failures.txt`. The open-copy case exposed an unavailable initial saved response being reused by opening instead of issuing its later read. Clearing that unavailable response restores the existing behavior in which a copy that becomes available during opening is shown.

The two resume fixtures omitted the current-generation checkpoint required by `docs/plans/session-resume.md` and verified by `TestAWholeCopyIsResumedWithoutAReplay`. Their saved responses now include their fixture generation and through-zero checkpoint. Missing and mismatched generations remain explicit negative cases in the new loading matrix. The steering-recovery case required an unfolded Working chip; it now verifies Working through the documented Window status popover when folding hides that chip. Its steering and attachment assertions are unchanged.

All five affected suites passed in 71 seconds in `.diagnostics/chat-loading-gate-corrections/`. That run also checks the spacing correction between the team link and membership error message. The historical saved-restore and click failures remain unexplained; these different, diagnosed gate failures do not resolve them.


The corrected full gate passed 158 of 159 suites in 271 seconds. All loading, restoration, resume and steering suites passed. The extension suite failed its existing repeated menu-entry focus-return assertion, reporting child BODY focus while host focus was on the iframe. The identical extension test (SHA-256 `45f9eb93408af8795232043da5f8694f74b6cf289f3667850a69380cb6fd1e8a`) passed in isolation on both committed baseline `cbd5766` and the candidate. Those isolated passes do not establish the full-gate failure's cause. Its output remains in `.diagnostics/chat-loading-full-corrected/failures.txt` as an unexplained diagnostic.

A separate controlled close-during-display run released the saved read, closed the window during its bounded construction, then reopened it before completion. It passed the same content, title/member, journal, export-control and Stop checks without a duplicate provider request or page error. The reproduction source and result are retained in `.diagnostics/chat-loading-close-during-display.js` and `.diagnostics/chat-loading-close-during-display/`.


The unchanged full confirmation run passed all 159 suites in 260 seconds, with an empty `.diagnostics/chat-loading-full-confirmation/failures.txt`. This pass does not establish the cause of the earlier extension focus failure.

Final metadata review found that session Info always serializes `title`, including an empty value. An added cached-title frame test reproduced a transient Untitled Chat title during loading even though the immediate assertion still saw the old title. The capture is retained in `.diagnostics/chat-loading-empty-title-before/`. An empty title now preserves the cached window name while loading/restoring; a persisted nonempty user title or authoritative nonempty session title can still update it. The known-title helper and registration use that distinction. This small correction follows the 159-suite pass and has its own focused gate below; the earlier full pass must not be described as testing this final correction.


The final correction passed seven affected suites in 86 seconds in `.diagnostics/chat-loading-empty-title-fixed/`: loading interval, open copy, resumed chat, saved restore, steering recovery, taskbar stacks and workbench. JavaScript syntax and whitespace checks are clean. No unrelated product or suite change followed this focused pass. The combined candidate includes Engine's saved-history export commit; it remains a review candidate until UI verifies the generated file's reveal and usable full-history access.


## Review follow-up for generation changes

Architecture requested separate coverage of a generation change after bounded saved rendering has started, and a generation change while zero-based fallback catch-up is in progress. The existing changed mode covers only a change during the fresh saved read.

The render-generation fixture wraps the existing saved-history scheduler step, lets its first batch construct rows, then delivers the new session generation before the old copy finishes. Its old snapshot carries an identifiable marker. The fallback-generation fixture delivers a new session generation while the old zero-based journal request is unresolved, then returns that old generation's marked entries. Both cases require a fresh current-generation snapshot, catch-up after its checkpoint, exactly one copy of new-generation content and no sampled frame showing the obsolete marker. State is delivered at the controlled boundary through the page's own event dispatcher, so these cases do not depend on an arbitrary timer.

This follow-up changes only the generation-race coverage and the correction it establishes. UI's original review remains scoped to committed `cc34562`; it released the shared slot before these checks started. The 159-suite result remains evidence from before the final title correction, and the seven-suite result covers that correction. These new cases have their own results below.


The initial expanded matrix ran against unchanged `cc34562` product code. The render-generation case passed, exercising the existing guard after the old copy completed. The fallback-generation case failed all three new assertions: it did not read a fresh current snapshot, did not replace the obsolete copy/checkpoint, and exposed marked old-generation journal rows on sampled frames. The 86-second run and failure output are retained in `.diagnostics/chat-loading-generation-before/`.

The correction rechecks generation after fallback catch-up and after bounded parent presentation is prepared. If either asynchronous step crossed a generation boundary, opening reads a fresh snapshot and establishes the current position before attaching the replacement. The saved display remains in sight while this happens. No export, bridge, saved-file or provider policy changes are needed.


The first corrected nine-suite run passed eight suites and both new generation cases, but its existing changed mode failed the one-event count. The new guard legitimately reread the fresh snapshot, and the fixture emitted a second distinct event with the same AFTER-SNAPSHOT marker on every read. The fixture now schedules that one in-flight event once per generation, retaining the one-copy assertion. The initial 113-second result is retained in `.diagnostics/chat-loading-generation-fixed/`; no production change followed that run.


The confirmation run passed all nine focused suites in 110 seconds in `.diagnostics/chat-loading-generation-confirmation/`. It covers twelve loading modes, including both generation races, plus background membership, running catch-up, sequence ordering, resumed display/following, open copy, saved restoration and recovery. Neither new race showed an obsolete-generation row on any sampled frame. Syntax and whitespace checks are clean. Only `session-lifecycle.js`, `e2e/chat-loading-interval.js` and this report changed after `cc34562`; no bridge or export code changed. This focused result covers the successor's correction. The historical 159-suite pass remains separately scoped and is not a full-suite claim for the successor.


UI additionally requested explicit close/reopen after the generation race has settled. Both race modes now close and reopen that settled window, then verify that old markers remain absent and every new-generation marker appears once. This supplements the existing close/reopen during opening and retains whole-interval stale-frame checks. The follow-up changes only the matrix and this report; product code remains `5eb2b3f`.


The first post-race reopen run was interrupted for shared-slot ownership. UI had announced its review in message 2242 before Front End saw that notice. Front End's wrapper acquired the file lease between UI replay cases. Architecture directed Front End to stop only its owned run. Its command line and output directory identified wrapper PID 29772 and child node PID 54328; Process.Kill(true) stopped that verified tree, and both PIDs were absent afterward. No lock file or UI process was removed. The follow-up output is incomplete, with run identity retained in `.diagnostics/chat-loading-generation-reopen/terminated-owned-run.json`. It remains queued until UI explicitly releases the slot. The earlier nine-suite pass for `5eb2b3f` is unchanged.


After UI explicitly released the slot in message 2250, the post-race close/reopen matrix passed all twelve modes in 73 seconds. The output and frame capture are in `.diagnostics/chat-loading-generation-reopen-complete/`. Both generation cases retained only the new generation after reopening, with each current question/answer and identified in-flight event present once. No sampled frame showed an obsolete marker. This follow-up changes only test coverage and this report; product code remains the UI-reviewed `5eb2b3f`.

UI independently passed the twelve-mode product review with 1,350 frames and no page errors before this extra test. External viewing remains unverified because native input returned Access is denied. The real-action export and Explorer reveal passed, but those results do not substitute for usable oldest, middle and newest entry viewing. Release still requires that check and its full gate on the combined successor.
