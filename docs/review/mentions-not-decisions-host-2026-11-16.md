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

The DTO and expanded forwarding control pass together once with race/all-vet and explicit runtime/debug crash capture in 12.065 seconds, exit zero. The crash file for that process is empty. The receipt is dinah-mention-dto-diagnostic.jsonl. A diagnostic ten-repeat run is now active on the final source. Full package receiving and the unchanged 18-case independent gate remain pending. No merge, push or installation is claimed.
