# Reopening Group Chats at Start and Showing Messages That Wait for a Replay

This is the shape of the third item in [Group Chat Work After the Cleanup](group-chat.md), written by Dinah Engine Cleanup on 29 October 2026 at Architecture's request, before any code. It has two parts. The first reopens the chats that belong to a group, in the background, when the window starts, and bounds what that costs. The second shows a message that is waiting for a chat still replaying its saved conversation as arriving, at once, instead of after the replay. The DESIGN.md changes are listed at the end and are made in the commit with this document. No code for either part exists.

## What the code does today

A chat that is not open is opened for a message by the watcher (`ConnectWaking`, once a second). It asks the bus which chats have asking messages waiting (`Waking`), and each one goes through the scheduler in `internal/bushost/scheduler.go`. A chat with a window is reconnected in its window. A chat without one is queued and opened in the background (`chat:connect` with `background` set), at most three at a time (`BackgroundSlots`), and each closes itself thirty minutes after its last turn ends (`BackgroundIdle`). A chat that fails to connect is not asked for again until a newer message wakes it. Nothing opens a chat that has no message waiting, so the first mention to a group's closed member pays for the whole reopen, and the message waits for it.

Delivery to an open chat is the window's `Ring`, also once a second. When the chat is not ready, which includes the time it spends restoring its options and replaying the saved conversation, `Ring` returns without taking the message. The message stays on the bus, and the person sees nothing of it until the replay ends and the next `Ring` delivers it into a turn. For a long chat that is many seconds in which the chat looks as though nothing has arrived.

## Part 1: reopening group members at start

**Which chats.** Once, after the page has reported which chats have windows (`ChatWindows`) and the window is not waiting to restart, the watcher asks the bus for the groups, newest activity first, and takes each member chat in turn, skipping the person, a session the window has no chat for, a hidden chat, and a chat that is already open or has a window (the layout restores those). A chat in two groups is taken once, at the place of the more recent group. The bus gives each member its workspace folder, which `RoomMember` does not carry today, so it gains a `Path`.

**The bound.** Each of these is a constant in `bushost`, and Architecture can change them.

1. **At most eight chats are warmed, once per start.** The list is cut after the eighth, in the order above. A window that starts with more group members than that leaves the rest to the existing path, which opens a chat when a mention for it waits.
2. **Two connect at a time (`WarmConcurrency`).** Starting the window does not start eight agent sessions at once. Each warm connect completes or fails (the existing two-minute patience) before the next starts.
3. **Warm chats do not use the three background slots.** Those slots are for chats opened by a message, and eight warm chats must not make a mention for a ninth wait thirty minutes for one to close. The scheduler keeps the warm queue and its pending requests apart from `BackgroundSlots`, and a conversation opened this way is marked warm so `BackgroundUsedLocked` leaves it out of the slot count.
4. **They close like any background chat.** Thirty minutes after the last turn ends, which for a chat that never took a turn is thirty minutes after it connected, the chat parks: it leaves the bus, stays in its groups, keeps its session with the agent, and the existing path reopens it when a mention waits. Warming is therefore a head start for the first half hour after the window starts, not a permanent cost. A chat that takes a turn, or whose window the person opens, behaves as any chat does.
5. **A failure is not retried.** A warm connect that fails is logged once and the chat is left to the message-driven path, which asks again when a newer message wakes it. There is no retry loop at start.
6. **A chat that cannot be connected for a standing reason is skipped,** for example a provider that is not signed in, by the same check that the message-driven path makes.

**What it costs.** Each warmed chat is a session with its provider, and for a provider whose adapter starts a process for each session it is a process. The cost was measured on 29 October 2026 on the machine the window runs on, from the window's own Claude chats: the ten agent processes the window started within three minutes of its own start held between 220 and 360 MB of working set each, 270 MB on average, with the Codex adapter at 237 MB. Eight warm Claude chats are therefore about 2 GB, and the window itself held 745 MB. This is an observation of chats that had been in use, not a controlled measurement of idle ones, and a chat that is only reopened may hold less; the limit of eight stands on this figure.

**Where it goes.** `internal/bus/rooms.go` (`Path` on `RoomMember`), `internal/bushost/scheduler.go` (`Warm`, the warm queue and pending, and `Next` honouring the two limits), `internal/sessionhost/bus_chats.go` (`WarmGroups`, called once from `ConnectWaking`), `internal/sessionhost/runtime.go` and `activity.go` (the warm mark and the slot count), and the window's `chat:connect` handler needs no change since the event is the one the message-driven path sends.

## Part 2: a message that waits for a replay shows at once

**What the person sees.** As soon as a chat that has a waiting message is open but not ready, the chat shows the message as arriving: a card at the end of the conversation with who it is from, the group it came in if any, and the first part of its text, labelled as waiting for the chat to finish reconnecting. When the chat is ready and the message is delivered into a turn, the card is replaced by the delivered message as it is now.

**How the engine does it.**

1. A new read-only `Bus.Arriving(ctx, session, limit)` returns the messages that `Doorbell` would count for that session, with their sender, group and text, and changes nothing. It shares the doorbell's condition (hop limit, budget, asking, not a question) so what is shown as arriving is what will be delivered.
2. In `Ring`, where it now returns for a chat that is not ready, it first checks whether the chat is open and restoring or replaying, which are the two reasons that last long. If so it calls `Arriving` and emits one chat event, `agent:arriving`, for the messages it has not yet announced for this conversation. The event is `{sessionId, messages: [{id, from, group, text, sentAt}], more}`, with the text cut at 400 characters and at most five messages, `more` counting the rest.
3. The event goes through the chat event log (`EmitChat`), so a window opened in the middle of the replay shows the cards from the log's head, and a window that was already open shows them as they come.
4. The announced message numbers are kept on the conversation and cleared when a delivery takes them, so each message is announced once and a later delivery does not announce it again. They are kept in memory only, since after a restart `Ring` announces them again from the bus.
5. A chat that is not yet a conversation, because its connect has not started, has nothing to show on. The connect begins within a second of the message and the conversation exists before the replay does, so the gap is at most a second or two.

**What the page does** is Front End's. The event is a chat event like the others. The card is replaced by the `agent:sent` event whose `delivered` entries carry the same message numbers (`DeliveredMessage.ID`), and a card whose chat closes or fails to connect stays with the note that it is waiting, since the message does.

**Where it goes.** `internal/bus/bus.go` (`Arriving`), `internal/sessionhost/bus_delivery.go` (`Ring`), `activity.go` (the announced set), `chat_events.go` (the event name).

## Tests

1. A group with four members, a window that has just started: the first eight are queued in recency order, a chat in two groups once, and hidden chats, chats with windows and open chats are skipped.
2. Only two warm connects are pending at once, and a third starts when one completes or fails.
3. Warm connects do not count against the three message-driven slots: with eight warm chats live, a mention for a ninth chat is opened at once.
4. A warm connect that fails is not asked for again until a newer message wakes the chat.
5. The warm pass runs once per start, not on every watcher pass, and not while the window waits to restart.
6. A message waiting for a chat that is replaying produces one `agent:arriving` event with its sender, group and cut text, a second `Ring` produces none, and a second message produces one for itself only.
7. More than five waiting messages announce five and count the rest.
8. When the chat is ready the message is delivered once and the announced set is cleared, so a later message from the same sender is announced.
9. A window opened mid-replay reads the arriving event from the chat event log.
10. `Arriving` changes nothing: the same messages are still counted by `Doorbell` afterwards, and it applies the hop limit and a group's budget as `Doorbell` does.
11. A mutation check: with the warm chats counted in the slots, test 3 fails; with the announced set not cleared, test 8 fails.

## DESIGN.md changes

- The paragraph on messages reaching a chat that is not connected says that, at start, the chats of groups are opened in the background ahead of any message, up to eight, two at a time and outside the three slots, and close like any background chat.
- The same paragraph says that a message waiting for a chat that is replaying is shown as arriving at once, and replaced by the delivered message when the chat is ready.
- The decisions table gets a row for the date.

## Questions for Architecture

1. Are eight chats, two at a time, outside the three slots, the right bound to propose, given that I will measure the cost and report it?
2. Should a chat that was warmed and took no turn close sooner than thirty minutes, since the person did not ask for it? I chose to keep one rule for every background chat.
3. Is a card in the transcript the right way to show an arriving message, or does Front End want it in the composer area, since the message is not yet part of the conversation?

## Ruling

Architecture approved this shape on 29 October 2026 (message 577).

1. **Eight chats, two connecting at a time, outside the three background slots, is accepted,** with one addition: only groups with a message in the last seven days are considered, so a group nobody has used for weeks does not start agents at every launch. The measured memory of a window with eight warm chats goes in the commit's notes, and the eight moves if the figure calls for it.
2. **A warmed chat keeps the one thirty-minute rule.** One rule for every background chat is easier to reason about than a second clock for chats the person did not open, and thirty idle minutes is already the cost Paul accepted.
3. **An arriving message is a card at the end of the transcript, marked as waiting,** in the place where the delivered message will appear, and the delivered message replaces it there. It is display, not input, so it sits in the transcript rather than near the composer, and a waiting glyph with its colour says what it is. Putting it where it will land means nothing jumps when it is delivered.
4. `agent:arriving` joins the numbered chat events, so it goes through `EmitChat` like the other five, and the source-scan test lists it.

## Change of 29 October 2026

Paul answered the question about the cost (message 593) with four, the most recent ones, and Architecture recorded it (message 594). `WarmChats` is therefore 4 in `internal/bushost/scheduler.go`, and the chats are still taken from the most recently active group first. Eight warm Claude chats came to about 2 GB by the measure above, and four come to about 1 GB. Everything else in the shape is unchanged.
