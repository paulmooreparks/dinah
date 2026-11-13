# A Closed Team Member Was Not Woken by Its Assignment

Paul reported this fault on 13 November 2026 and asked for it to be filed and fixed. It is open, and nobody has diagnosed it yet.

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
