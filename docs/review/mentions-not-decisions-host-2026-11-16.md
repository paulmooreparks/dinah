# Mention routing host change

This standalone branch starts at master 6aa42180 and implements Paul's ruling in Lead message 10127. It precedes the remaining six-source settlement host. Front End owns notification navigation and rendered receiving; Quality froze 18 independent cases at 58285293 without reading this implementation.

## Behavior

The Decisions predicate now starts only from person_requests. Plain mention messages cannot enter its list or count. Their old detail addresses return an explanation that the message belongs to its chat. No migration deletes or rewrites the underlying messages, chats, membership, attachments, receipts or answer-origin records. Existing closed answer-origin compatibility tests remain unchanged.

AcknowledgeMention(messageId) is a trusted window method, registered as page work. It is absent from service and chat ports. The bus checks the actual message's human delivery, noncancelled status, agent sender, text kind and absence of a structured person request. Invalid, unaddressed and structured identities are refused. Acknowledgement is stored in mention_acknowledgements with a unique message key. It changes neither Inbox read/dismiss state nor the source message and sends nothing to the asking chat. A repeated acknowledgement succeeds without another write.

The watcher excludes acknowledged messages from new notification capture and clears an already stored card from the same durable evidence. This repairs interruption between acknowledgement and projection update. Actual replies to the message still clear attention, and later person-authored text in its group clears preceding plain mentions there. Other groups and structured requests remain independent. The notification DTO retains kind mention, group identity, original message ID and original sender identities.

The trusted page calls acknowledgement only after exact-message navigation succeeds. The host does not infer arrival from background read receipts, current focus or chat titles. Front End is responsible for observed arrival and marking; the host accepts the person's explicit window action. The page must retain the notification on a failed open.

Front End found that structured requests also use kind mention on their existing cards (10175). The additive requestId now comes directly from person_requests during live capture and every saved Notifications read. Old saved cards are enriched from that source without rewriting chat records or trusting a saved string. A source read failure propagates rather than classifying the request as plain. The Notifications store lock is released after its detached read and before bus lookup. The new actual live-event and old-card control passes without consulting the Decisions index.

## Existing expectation changes

The original focused receipt is retained privately as dinah-mention-boundary-initial.jsonl. It records the former plain-answer-origin test indexing an empty receipt list, and DecisionEntriesReuseDurableAttention expecting a second Decisions row for a mention. These are authorized behavior changes under Paul's ruling, not weakened receiving assertions.

The old plain-mention origin test now requires refusal of a Decisions answer, sends its explicit invented reply through the ordinary chat route and verifies that this ordinary reply has no Decisions answer origin. The legacy serialized origin compatibility test is untouched. Group-request answer coverage stays in place while its plain mentions remain outside the list. The old mention paging fixture now requires only its actual structured request; full request paging remains covered by DecisionEntriesCompleteAndPartialLifecycle. The first-start/backlog checks exclude all plain mentions, and their query-plan check recognizes the existing person_requests_open index used by the new candidate query. Old mention dismissal receipt checks now require a durable rejection, including repeat equality. The desktop entry test retains request/card behavior and uses navigation acknowledgement for the separate mention card.

After these explicit amendments, the focused race/all-vet selection passes both packages: bus in 34.125 seconds and desktop in 18.295 seconds. That receipt is dinah-mention-boundary-corrected.jsonl. The new navigation and group-reply controls preserve original message bytes, reject wrong/structured IDs, leave the other mention present and retain the actual structured request.

## Real-App receiving seam

The existing org_chart_acceptance/startup_page_acceptance fixture supplies isolated homes, real App/bus stores, counted peers, titled documents, dirty drafts and protected artifact snapshots. New tagged methods are registered only in that fixture:

- OrgChartHostFixtureMentionSeed accepts kind direct, group, request or nonmention, plus optional fixture chatId, group and text. It sends through actual bus methods and captures attention with the actual watcher. It returns the original message and structured request where applicable.
- OrgChartHostFixtureMentionState rereads actual notifications, Decisions entries/list, source messages, persisted notification bytes and pending task questions. It does not synthesize success flags.
- OrgChartHostFixtureAcknowledgeMention forwards the original message ID to the current actual App after restart.
- OrgChartHostFixtureMentionCall forwards a bounded set of production query/navigation/reply/acknowledgement methods with original JSON arguments and actual results. It validates the argument count and uses the current App after the existing restart control.

The seed control validates fixture chat and group identities. It rejects arbitrary identities. Existing fixture Control and Snapshot retain their separate uses. The page binding must record its raw requests/results, source commit, tagged binary hash, navigation timing and actual DOM target rather than treating these ports as a substitute for rendered evidence. Legacy/typed question positives can use the existing real typed-question fixture; no task mutation is added by this host change.

The initial tagged seed/acknowledgement/restart test passes with race/all-vet in 3.507 seconds. It proves that two plain mentions, a structured request and an unaddressed group message enter the actual stores; invalid acknowledgements fail, the acknowledged card stays cleared after a real App restart, and the other mention and request remain.

The expanded ten-repeat command stalled during a receiving restart, leaving an incomplete JSON record. It exceeded its internal timeout without a usable traceback. Its verified owned process tree was stopped, and the queued broad gate was cancelled before starting. That run remains failed evidence in dinah-mention-final-repeats.jsonl; it cannot be replaced by the earlier pass. No cause is inferred from the incomplete output.

The DTO and expanded forwarding control pass together once with race/all-vet and explicit runtime/debug crash capture in 12.065 seconds, exit zero. The crash file for that process is empty. The receipt is dinah-mention-dto-diagnostic.jsonl. The diagnostic ten-repeat run then passes all four controls in 79.462 seconds with exit zero and an empty crash file. Its receipt is dinah-mention-dto-repeat-diagnostic.jsonl. It does not erase the prior stalled gate.

Quality's source review in 10197 accepts the additive requestId distinction, without a runtime run. The full desktop, bus and task-host package command is active on the final source. The unchanged 18-case independent gate remains pending. No merge, push or installation is claimed.

## Task-positive receiving extension and broad package result

The full package command completed with desktop passing in 274.960 seconds and taskhost passing in 0.891 seconds. Bus failed only its two historical scan assertions: the previous-row comparison still included plain mentions, and the query-plan check required the old deliveries index. The amended comparison retains every previous structured-request row, excludes plain mentions under Paul's ruling and checks the existing person_requests_open index. The original full receipt remains dinah-mention-final-full.jsonl. A separate full bus rerun is pending.

The receiving extension adds OrgChartHostFixtureMentionTask(kind), where kind is legacy, text or multiple. It is compiled only with org_chart_acceptance, startup_page_acceptance, quality_task_host and quality_typed_questions. It uses the existing frozen consumer's durable generation/hold and the actual typed-question Review, Begin and Start path. The isolated setup approves its review consent through the real window Answer method and returns the actual pending question. PendingTaskPermissions and the existing receiving state read expose the resulting questions. No synthetic task records or Decisions results are supplied.

The first task-positive control failed because the fixture App had not started its window task services. The extension now starts those actual services before finding its private consumer. The retained corrected command passes the task-positive and existing seed/restart controls with race/all-vet in 10.977 seconds, exit zero. Receipts are dinah-mention-task-seam.jsonl and dinah-mention-task-seam-corrected.jsonl. The linker reported its existing manifest merge warning; neither the test result nor race detector reported a failure in the corrected run. This extension changes tagged test code only. Quality's composed receiving and the page's actual navigation remain pending.

## Current-base handoff

The host is rebased onto shipped master fe6b8eb3. Range-diff confirms identical production patches; the shared test fixture conflict retains both cleanup and mention fields and methods. The current-base focused mention controls pass with race/all-vet in 8.808 seconds. The complete profile/public-name guard passes in 7.783 seconds, including the private forbidden-term scan. Receipts are dinah-mention-rebased-focused.jsonl and dinah-mention-rebased-public-guard.jsonl. The earlier guard failure on the old base is retained in dinah-mention-public-guard.jsonl; all reported locations were neutralized by master's 4b8bf5ef and are unrelated to this change.

The full bus rerun after the two explicit scan corrections passes in 45.299 seconds (dinah-mention-bus-final.jsonl). The earlier full desktop and taskhost results remain preceding-base producer evidence. The composed independent 18-case gate and both-product page receiving are pending with Quality and Front End. This standalone host is ready for their receiving and Lead's integration gate, without any claim of installed acceptance. No test runner remains in this tree.

## Empty initial notification observation

Quality's first independent host run (10234) refused all seven selected cases before seeding because the fixture's raw notification-file observation required notifications.json to exist. An untouched store legitimately has no such file. The tagged observation now returns empty bytes only for os.ErrNotExist and still propagates every other read error. No production code changes. The actual seed/restart control now reads and checks this empty baseline before sending anything, then retains its existing acknowledgement and restart checks. It passes with race/all-vet in 3.666 seconds, exit zero, in dinah-mention-empty-baseline.jsonl. Quality retains its original seven failures; composed independent receiving must rerun on this child.

## Positive partial-answer and saved-draft setup

Quality's decision-sources preservation case (10251) required a nonempty baseline. The tagged seed now accepts two-question-request, with both actual questions initially pending, and partial-request, which also commits question one's invented answer through AnswerDecision. MentionCall now forwards AnswerDecision, ReadUIState and SaveUIState with their original arguments against the current App. These are isolated setup/read ports, with no production API change.

The positive control seeds two questions, answers only the first through the forwarded production method, saves a nonempty question-two draft through SaveUIState, reads projections and restarts the actual App. The existing answer remains and question two stays pending; the saved draft remains equal. Its first assertion incorrectly compared formatted json.RawMessage bytes to their compact input. The corrected assertion compares JSON values through their canonical encoding. Both receipts are retained as dinah-mention-partial-draft.jsonl and dinah-mention-partial-draft-corrected.jsonl. The corrected control passes race/all-vet in 3.366 seconds, exit zero. Independent receiving remains pending.
