# Keeping a Chat's Bus Identity Across a Provider Change

This is the shape of the first item in [Group Chat Work After the Cleanup](group-chat.md), written by Dinah Engine Cleanup on 29 October 2026 at Architecture's request, before any code. It answers the three questions Architecture asked: where in the continuation's commit the identity moves, how a switch that is rolled back leaves it with the old session, and how the old session stops answering to it before the new one starts. No code for it exists.

## What the code does today

A chat's place on the bus is a bus session ID such as `window-ABC…`. It is held in three places, and a switch has to treat each of them.

1. **The bus's own records.** The session row, the group memberships, the links that trust it and the messages waiting for it are all keyed by that ID. These do not change, as the plan says.
2. **`Settings.BusSessions[chat]`.** The window's map from a chat to its bus ID. `chatOfBus`, the wake code and the group lists use it to turn a bus ID into a chat.
3. **The conversation's own `busID`** (`Activity.busID`), set by `JoinBus`. `BusChats.Ring` rings every conversation that has one, and `LeaveBus`, which a close calls, ends the bus session with `Bus.EndSession`.

`SessionLifecycle.New` always makes a fresh ID (`BusSessionID("")`), registers it on the bus, and hands it to the agent as the `--session` argument of the chat's bus helper, which the agent starts with the session. After the session exists, `JoinBus` records it in the conversation and in `BusSessions`. `Continuations.Continue` calls `New`, so a switched chat gets a new identity, and the old identity stays on the bus, in its groups, for a chat that no longer answers.

Two facts decide the design.

- **The new session's helper is fixed when the new session is created.** Its `--session` argument cannot change afterwards, and the bus does not change either, so the new session has to be created under the old identity from the start. Re-binding it at the commit is not possible.
- **The old conversation can end the identity later.** When the old conversation closes, `LeaveBus` reads its `busID` and calls `EndSession`, which sets `ended_at` on the row the new chat is now using. And until its `busID` is cleared, `Ring` can ring the old conversation as well as the new one.

## The shape

**1. The new session adopts the identity when it is created.** `New` gets an optional adopted bus ID. With one, it uses that ID in place of a fresh one, registers it (`RegisterSession` on an existing row refreshes `last_seen`, clears `ended_at`, resets `stop_continues` and keeps the title when none is given, which is what any resume does), and serves it to the new agent session. An adopting `New` does not call `JoinBus`, does not save a mapping, and does not end the identity if it fails, because the old session still owns it. A chat with no bus identity yet has nothing to adopt and takes a fresh one as now.

**2. The identity moves inside the continuation's commit, after the handoff is durable.** In `Continue`, the existing commit point is `HistoryBeginHandoffLocked`, which writes the new chat's transcript with its link to the old one. The transfer comes straight after it succeeds and before `RevokeSession` and `SetSuperseded`. In that order:

1. `old.ReleaseBus()` clears the old conversation's `busID` and title state under its mutex, returns the ID, and does not call `EndSession`. From here the old conversation does not answer to the identity and can never end it.
2. `next.JoinBus(id)` gives it to the new conversation, in memory.
3. One `updateSettings` sets `BusSessions[next]` to the ID and deletes `BusSessions[old]`. One write moves the mapping, so there is no state on disk in which both chats or neither have it.
4. After the commit, `PublishBusTitle(next)` puts the title on the bus. This is best effort, as it is today.

Because the transfer comes after the durable handoff link and before the in-memory supersede, a restart never finds the identity moved without the continuation, and never finds the continuation without a way to tell which chat owns the identity.

**3. A rolled-back switch leaves the identity with the old session.** Every failure before the transfer leaves it untouched, because the old chat still owns it and the adopting `New` neither joined nor ended it. That covers the revision checks, the guards and the handoff write. If the settings write in step 3 fails, the transfer is undone: `next.ReleaseBus()`, `old.JoinBus(id)`, then the existing `cleanup()` and the error, with the settings as they were. `cleanup()` closes the new conversation, which calls `LeaveBus`, so it only reaches the bus if the new conversation still holds the ID. It does not, in either case, because it either never joined or was released first.

**4. The old session stops answering before the new one can.** Step 1 comes before step 2, so there is no moment at which both conversations hold the ID. Between them nobody does, and messages for it wait on the bus, which keeps them durably, until the new chat joins. Both conversations are also marked as switching until the commit ends, and `next.SetSwitching(false)` comes after the transfer, so neither could start a turn from a bus message during it. Afterwards the old conversation is superseded and revoked. The old agent's bus helper process still runs with the ID in its arguments, but the old session is never prompted again, so it never calls a bus tool.

## What this does not fix

- The bus's `agent_sessions` rows bind an agent session to the bus session for steering hooks. The old agent session's row stays until the bus session ends. It only matters if the old agent fires a hook, which a revoked, never-prompted session does not.
- A switch does not rename anything on the bus beyond the title the existing code already publishes.

## Tests

1. A switch moves the identity: the group membership, a waiting message and a link that trusts it are on the bus as before, `BusSessions` has the new chat and not the old, the old conversation's `BusID` is empty, the new one's is the ID, and a message sent to the ID after the switch starts a turn in the new chat and not the old.
2. A rolled-back switch, with the failure injected at each step (the guard, the handoff, the settings write), leaves the old chat with the ID in memory and in settings, the new chat without it, and the bus row not ended.
3. After a switch, closing the old chat does not end the identity.
4. An adopting `New` that fails does not end the identity.
5. A bus message that arrives during the switch waits and is delivered once, to the new chat.
6. A mutation check: with the release moved after the join, or after the supersede, tests 1 and 5 fail.

## Where it goes

`internal/sessionhost/session_lifecycle.go` (`New` with an adopted ID), `internal/sessionhost/conversation_ops.go` (`ReleaseBus`), `internal/sessionhost/bus_chats.go` (the transfer), `internal/sessionhost/continuation.go` (`Continue` and the recovery path that shares it), and the window's glue for the one settings write. The recovery path, `Recover`, calls `Continue` with the recovery flag, so it takes the same transfer.

## Questions for Architecture

1. Is it acceptable that adopting resets `stop_continues` on the identity's row before the switch commits, as any reopen does? A rolled-back switch then leaves it at zero.
2. Should a switch to a provider that cannot take the bus helper at all (no `WithBusServer` result) keep the identity with the old chat, so that the chat is not left without one? I would say yes, and refuse the transfer.

## Ruling

Architecture approved this shape on 29 October 2026 (message 561).

1. **Resetting `stop_continues` when the new session adopts the identity is accepted.** Any reopen resets it, and a rolled-back switch leaving it at zero costs at most one extra automatic continuation.
2. **A switch whose new session cannot take the bus fails as a whole, before the commit.** In the window a chat goes without the bus only when registering it fails (`withBusServer` logs the error and opens the chat anyway), not because of its provider. Refusing only the transfer would leave the identity with a superseded chat that never answers, which is the defect this work removes. The adopting `New` therefore reports the failure, the switch rolls back through the existing cleanup, the old chat keeps its identity and stays the live chat, and the person is told the switch did not happen and can try again.
3. **The new agent session's steering binding is created as for any new session.** The old agent session's `agent_sessions` row may stay as the document says, but the commit has to show that the new agent's hooks resolve to the adopted identity, with a test.

The order in step 2 of the shape (release, then join, then one settings write) and the mutation check stand as written.
