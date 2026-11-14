# Decisions backlog correction

The candidate excludes historical plain mentions and closes earlier plain mentions when the person next speaks in their group. On a fresh protected backup with 7,467 messages, Decisions showed zero rows. Its entry index and status count also showed zero. Attention still imported five unanswered unread messages into four cards.

## Policy and implementation

The branch `quality/decisions-backlog` starts at master `4cd9b954`. The policy is recorded in `docs/design/decisions.md`.

The first bus open with this policy creates one `decision_start` row containing a UTC timestamp and the highest committed message ID. Subsequent opens preserve both fields. The ID is the ordering boundary, so messages with identical timestamps cannot cross it accidentally. Homes which used the earlier implementation have no such row; their first open with this correction establishes the boundary. There is no attempt to infer an earlier first-use time from unrelated files.

The shared `openDecisionSQL` predicate supplies the list, individual mention resolution and `DecisionEntries`, whose count feeds the status bar. It considers open structured requests independently of the boundary. Plain mentions require a message beyond the boundary, an active human delivery, no explicit dismissal or direct human reply, and no later person-authored message in the same group. Ordinary group conversation does not close structured requests.

Candidate IDs come from an indexed message-ID range and an index on open structured requests. New indexes support human messages by group and delivery lookup by message. The query excludes old bodies before materializing them. Existing pagination remains at 100 rows. This does not impose a new retention limit on legitimate unanswered structured requests or future quiet-group mentions.

## Verification

The focused Decisions checks passed in 4.573 seconds. The complete bus package then passed in 35.754 seconds, including schema upgrade tests. The desktop Decisions tests passed in 1.411 seconds. All tests and the candidate build used the shared FIFO. No full browser suite ran.

New tests cover a first start after old mentions, preservation of that boundary across reopen, two earlier mentions closed by ordinary group conversation, an unanswered group, a fresh mention after the person speaks, structured requests surviving both rules, and agreement between list and entries. Existing tests retain direct-reply, dismissal, read-state, pagination and structured-answer coverage. An execution-plan assertion verifies an indexed message-ID range.

The 7,000-message fixture listed its remaining structured request in 1 ms. Its entries query measured 0 at the timer's resolution. There is no wall-clock threshold in the test that could fail merely because a machine is slow.

## Fresh-backup rehearsal

The candidate was built with `desktop,production` tags and tested without installation. Its executable SHA-256 is `99a8e625e6b79dc9a69dad39f9ba8789badc07cf8228426f3c212b71d5dcd67c`.

The original database was opened with SQLite `mode=ro` under a Windows low-integrity impersonation token. Its database and WAL sidecar labels were checked first. Only the fresh temporary destination received a low-integrity label. The SQLite backup API copied a consistent snapshot; no live database files were copied manually. The documented no-write-up boundary and invented-data proof were established in the preceding restart rehearsal. The reusable preparation tool is `scripts/restart-rehearsal-backup.py`.

No original home file was written. No separate credentials or token file was read or copied. Authentication, hook-token and preview rows were cleared from the temporary database without selecting their values. No saved runtime chats, queues or settings were copied. The candidate ran with isolated home and provider-config paths, an empty workspace and native notifications disabled. The only browser was headless Chromium with non-loopback page requests blocked.

| Observation | Result |
| --- | --- |
| Database backup | 79,015,936 bytes in 378.046 ms |
| Snapshot | 7,467 messages; 148 chats; 30,859 deliveries |
| Policy boundary before startup | Absent |
| Policy boundary after startup | One row at the snapshot's final message |
| Hidden entry ready | 709 ms from launch |
| UI state ready | 1,711 ms from launch, including browser startup |
| Attention import observed complete | 2,220 ms from launch |
| Decisions list, entry count and status count | Zero each |
| Attention | Five records grouped into four cards |
| Working chats and browser page errors | Zero each |
| Browser entry-index call | 102.4 ms including bridge overhead |
| Browser list plus notification-read phase | 784.8 ms including bridge overhead |
| Direct bus list on the copy | Below the timer's reported resolution |
| Direct bus entries on the copy | 0.5001 ms |

The direct bus measurements used this branch's package after the hidden process stopped. They isolate the query cost from browser and bridge work. These are single observations, not controlled before/after benchmark distributions. The earlier 461-row rehearsal and this snapshot contain different numbers of messages. The browser phase originally had an overly narrow metric name; the committed runner calls it `decisionAndNotificationReadMs`. Only that output label changed after the successful run.

Counts were unchanged for messages, deliveries, sessions, steers, steering submissions and structured requests. The complete message-row digest also matched. Startup sent no message in this isolated run. Actual recovery of Paul's live runtime chats is outside the rehearsal because their files were intentionally absent.

The temporary database, notifications, logs, private entry URL and browser state were deleted after the hidden process stopped. The temporary rehearsal directory no longer exists. Only aggregate receipts, invented-data test logs and the candidate executable remain in the temporary build folder. No real message, chat title, screenshot or page dump was retained in the report or repository.

There was no push, merge, installation, visible window or desktop input. Engine owns the underlying bus area and was notified of the changed policy, storage and indexes.
