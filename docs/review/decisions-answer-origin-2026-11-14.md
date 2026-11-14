# Decisions answer-origin verification

The implementation on `engine/decisions-origin` is `e17f3b11025fb3cab531edd1170be4548a51451b`. The contract was committed first at `2d6674eb`, with the arriving marker added at `551692c5`. Its current shape remains in `docs/design/decisions-answer-origin.md`.

AnswerDecision now records versioned question/answer snapshots in an additive `decision_answer_origins` table in the same transaction as its reply and receipt. Partial answers carry only the questions answered by that reply, in request order. Recommended answers retain their resolved words and choice flag. Plain mention answers carry question zero and the original mention's text. All strings use the existing redaction path. Ordinary replies and old messages have no metadata, and their prose is unchanged.

Message reads fetch the optional origin by its message primary key. Window and delivery DTOs carry cloned values, as do transcript and prompt copies. Arriving previews carry only `decisionAnswer:true`, within the existing text bound. DecisionByID reads closed and open requests without changing delivery or attention state. Its projection always has `readOnly:true` and advertises no dismissal action.

## Checks

All commands ran through FIFO against invented state.

| Check | Result |
| --- | --- |
| Full `internal/bus` suite | Passed in 41.126 seconds. |
| Full `internal/sessionhost` suite | Passed in 9.471 seconds. |
| Focused desktop Decisions, delivery, replay, relay and arrival suites | Passed in 15.723 seconds. |
| Desktop-tagged answer-origin integration | Passed in 0.886 seconds. |
| New origin tests across bus, runtime and desktop | Passed in 0.949, 0.507 and 0.596 seconds. |

The new tests verify partial and explicit/recommended snapshots, plain mentions, retries, transaction rollback, persisted origins, group reads, pending arrivals, delivery, nested-copy isolation, saved transcripts and provider replay. The desktop integration replays an additional unmatched prompt so that replay becomes the saved copy, and verifies that typed origins survive both that copy and the emitted page update. Closed lookup, invalid identities, query classification and unchanged change counters are checked. Existing tests were not weakened or edited.

The cross-version runner compiled yesterday's bus at `4fe5a52c791a949a54380250e3503ce84077a318`. That code created the database. The current code migrated it and added an invented answer-origin row on the trial reply. Yesterday's code then reopened it, delivered the reply and wrote another message. The current reader verified all messages, database integrity and the surviving origin row. Those stages passed in 0.387, 0.197, 0.185 and 0.166 seconds. The old worktree was left clean. The runner's optional CurrentTest argument leaves its previous default behavior intact.

No browser card, native window or real model was exercised for this backend task. The provider replay check uses the existing stand-in provider. Front End owns presentation and navigation against this candidate. No full browser suite, live database, real-home access, credential-file read, push, merge or installation occurred. Old native-window behavior was not retested; yesterday's transcript reader was inspected and uses ordinary JSON unmarshalling, which ignores the added optional field. An old build may omit the answer-card metadata if it rewrites a transcript, while the ordinary text and bus origin row remain.

The branch is ready for Lead and Front End to review and integrate together.
