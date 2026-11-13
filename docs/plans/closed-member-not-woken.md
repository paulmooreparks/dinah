# A Closed Team Member Was Not Woken by Its Assignment

Paul reported this fault on 13 November 2026 and asked for it to be filed and fixed. The cause is found and the first part of the fix is built; the section The Cause and the Fix, at the end, says which part is left.

## What Happened

Dinah Team Lead assigned work to its team member Dinah Front End with the action `dinah.team/assign@1`. The member's chat was closed. The assignment was posted in the team's group chat as message 6891, with the member as the one it asks to act, and a correction followed six seconds later as message 6892, addressed to the same member with the bus send tool. The member did nothing until Paul opened its window himself, and then it took both messages at once.

The product says the opposite will happen. The chat instructions every agent is given say that a message which asks a closed chat to answer opens it in the background, and the description of the assign action says that the message wakes the member.

## Evidence

The bus database has the times, in UTC.

| Event | Time |
| --- | --- |
| Message 6891 created (the assignment) | 15:40:44 |
| Message 6892 created (the correction) | 15:40:50 |
| Both delivered to the member's session | 15:41:56 |

Both messages have the member's bus session in their `mentions` column, so the bus knew whom they asked. Both deliveries have the same `delivered_at`, to the fraction of a second, which is also the session's `last_seen`. That is one delivery of everything waiting when the chat connected, 72 seconds after the first message, and it matches Paul opening the window. The other three members of the team, which the messages did not ask, have deliveries that are still waiting, as they should.

The member is a Codex chat, session `codex:01a1a469-e6d0-7502-9b05-a09bdece6701`, bus session `window-WNRUZSO24CLIY366OTPRJSUYBP`. `dinah.team/list@1` reported it as closed, with the reason "its chat is not open", a few minutes before the assignment. The window's log has no line about a wake for it in that minute, neither an attempt nor a refusal.

## Where to Look

`ConnectWaking` in `internal/sessionhost/bus_chats.go` is the routine that opens closed chats for messages. Three things in it can leave a chat closed without a word in the log, and each should be checked against this case.

1. The routine returns at once while the window's chats are not yet known, or while the window is holding turns for a restart. Turns are held only after the person chooses Restart to Update and the window waits for running turns to end (`holdingTurns` in `cmd/dinah-desktop/update.go`). The lead's own turn started in that minute, so this is unlikely to be the cause here, but in that state no closed chat is opened for a message and nothing tells the sender.
2. `Waking` on the bus decides which sessions have a message that asks for them. The messages' mentions are right, so the question is whether a closed member of a team is among the sessions it returns.
3. `ConsiderWakeLocked` can decline a chat that is hidden, that failed to connect lately, or that is over the limit on chats open in the background.

The member's task before this one ended with an instruction to stop work, so a state left by that, such as a parked or held chat, is worth ruling out too.

## What Fixed Means

A message that asks a closed chat to act opens that chat in the background and starts its turn, with no window opened by the person. When Dinah does not open the chat, for a reason it knows, the sender's receipt says so and says why, and the log has a line for it. The send receipt already warns when a chat runs outside the window and cannot be woken, and this is the same promise for a chat that the window chose not to open.

The check is a test that sends an assignment to a closed member and finds its turn started, and one for each reason a wake is declined that finds the reason in the receipt.

## The Cause and the Fix

The cause is the third of the places above. At most three chats are open in the background at once (`BackgroundSlots` in `internal/bushost/scheduler.go`), and a chat opened there kept its place for thirty minutes after its last turn ended. A chat that a message asked for while all three places were taken was put in a queue, and nothing was written or shown.

That was the state on 13 November. The chat list, read a few minutes after the fault, had exactly three chats open in the background: two that were working, and one that had been idle since 15:31 UTC, nine minutes before the assignment. The member waited behind a chat that had nothing to do.

Three things are changed.

1. A chat open in the background that has been idle for a minute gives its place to a chat that waits, longest idle first (`yieldIdleBackground` in `internal/sessionhost/bus_chats.go`, `BackgroundYield`). It is parked as an idle chat is, so it keeps its session with the agent and its next message opens it again. A chat that is working, has background work or subagents running, or ended a turn within the minute keeps its place.
2. When a chat still waits, because all three places belong to chats that are working, the log says so once, with the chat that waits and the three that are in its way.
3. The team's list (`dinah.team/list@1` and the team page) says beside such a member that a message waits for it and why.

`TestAnIdleBackgroundChatYieldsItsPlaceToAWaitingOne` covers the first and the third.

Two parts are left. The sender's receipt does not say that the chat it asked could not be opened: the receipt is written by the bus when the message is sent, and the places are known only to the window, a moment later. And the limit itself is still three, so a fourth chat still waits when three are working at once, for as long as they work. Whether three is the right number is Paul's to say.

## Engine follow-up, 14 November 2026

The remaining changes are prepared on engine/background-limit, from master 393c763a, in .worktrees/engine-background-limit. This branch is for review only; it is not merged, pushed or installed.

Settings > Chat now has Maximum Chats under Chats Opened in the Background. The application stores backgroundChatLimit in settings.json. A missing or invalid stored value uses eight, and the Settings method accepts integers from one to sixty-four. Reads, changes and wake admission share the application mutex. The next wake check applies a change without restarting. Lowering the limit keeps current chats and pending connects; another chat waits until admission is below the limit. Visible chats and startup-warmed chats retain their existing separate treatment.

When noteWaiting finds a queued chat after idle yielding and admission, it looks up the original senders of actionable, undelivered messages. Each sender receives one private host-result notice for that waiting chat, identifying it and explaining that the background places are in use. The original message stays queued. A later sender receives its own notice; more messages from an already-notified sender do not repeat it during the same wait. The person's notice goes to the bus inbox. These are bus messages, with no operating-system notification or desktop interaction introduced.

Notices use the bus's durable host-result keys based on target, sender and first pending message. Repeated checks or reconciliation after a restart do not duplicate a notice for that delivery. The in-memory sender set also suppresses another notice within the same waiting episode. A chat admitted by an idle yield is no longer queued before notification runs, so admission sends no second notice. Host results are excluded from sender lookup, preventing notification chains. The send receipt remains unchanged because it precedes this admission decision.

Go fixtures cover the default, persistence, live admission changes, preservation after lowering, configurable shortfall, sender privacy, human delivery, durable deduplication, passive group readers and notification-loop prevention. The application fixture assigns work through a group to a closed member, observes one sender notice while blocked, and then observes admission after a yield without another notice.

The first two focused executions failed in the new application fixture because it tried to address a closed chat directly through the live-chat address resolver. The corrected fixture uses the group-assignment route from this incident. The third focused execution passed. Full touched-package results and verification limits are recorded before handoff; no full browser suite or desktop automation is permitted for this branch.

The final complete Go suites passed for bus, bushost, sessionhost and desktop, including their subpackages. Focused race checks and JavaScript syntax checks also passed. The race link printed a Windows manifest-merge diagnostic despite returning zero and running the tests. docs/review/background-limit-2026-11-14.md records the commands, earlier fixture failures, results and unverified UI/build scope.
