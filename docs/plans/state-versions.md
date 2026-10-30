# Ordered State Events

This is the shape of the fifth item in [Group Chat Work After the Cleanup](group-chat.md), written by Dinah Engine Cleanup on 29 October 2026 at Architecture's request, before any code. It says which state events carry a version, where each owner's counter lives and how it is read under the owner's lock, and how a resync's reread fits with it. The DESIGN.md change is listed at the end and is made in the commit with this document. No code for it exists.

## The defect

Every engine event is emitted after its owner's lock is released. When two changes to one owner happen close together, their events can therefore reach the page in the opposite order to the one the changes happened in, and the page then shows the older state until the owner changes again. Provider state showed it. The state is stored under a lock and emitted after the lock is released, so one thread can store its state, a second can store a newer state and emit it, and the first then emits the older one last. The page ends on the older state while a reread of the stored states returns the newer. The same shape exists wherever an event carries a whole value: a session's information, its queued prompts and its options, a provider's account, the settings preferences and the update state.

The sequenced-events work in [event-sequence.md](event-sequence.md) does not cure this. It numbers events as the bridge sends them, so it fixes the order in which a page that reconnects replays them, but it numbers them in the order they were emitted and so keeps the inversion.

## The shape

**Each owner of a piece of state has a counter, and a state event carries a number taken from it while the value is read.** The page remembers the highest number it has applied for each owner and each kind of event, and ignores an event whose number is lower. No emit moves: the events still go out after the lock is released, in whatever order they go, and the page puts them back in order.

For this to be right, the number and the value must come from one critical section of the owner. If a number were taken first and the value read later, or the other way round, a number could describe a value from before or after it. So the counter is advanced inside the lock that guards the value, in the same acquisition that copies the value out, and the event is built from that copy.

**Events built from a copy made at emit time.** Some of today's emit sites build the value earlier, from a result an earlier call returned, and emit it later. Those sites change to read the owner's current value when they emit, through a function that returns the value and its number together. That makes the event carry the latest state at emit time, which is what a state event is meant to say. An older event that arrives after a newer one is then dropped, and the page ends on the highest number, which is the state read last. It is correct because every change to an owner is followed by an emit that reads the owner afresh.

## Which events and which counters

| Owner | Lock | Events | Where the counter lives |
| --- | --- | --- | --- |
| A conversation | its mutex, `c.mu` | `agent:session`, `agent:queue`, `agent:options` | a field of the conversation, advanced in `EmitState`, which holds `c.mu`, builds the payload from the `...Locked` readers and takes the number |
| A provider's lifecycle | `stateMu` | `agent:state` | a counter beside the state map, advanced in `state()` under `stateMu` where the state is stored |
| The window's accounts | `App.mu` | `agent:account` | a counter of the window, advanced where the account is stored |
| The window's settings | `App.mu` | `settings:changed` | the same counter of the window, advanced in `emitSettingsChanged`, which already copies the snapshot under `App.mu` |
| The update watcher | `updMu` | `update:state` | a counter beside the update state, advanced in `UpdateState` under `updMu` |
| The judgements' state | `numMu` of the window's judging | `judgements:changed` | a counter of its own, advanced in `JudgementState` in the same step that reads the day's summary, so the event and the call that serves the same value are numbered alike |

All of these use one small type, `StateClock`, in `internal/sessionhost`: a random generation chosen when the owner is made, and an atomic counter. It is only ever advanced while the owner's lock is held, so the atomic is for the readers who copy it, not for ordering.

Each versioned event gets two fields, `stateGen` and `stateVer`. They are named so as not to collide with the `gen` and `n` of the numbered chat events, which the page's sequencer treats as a stream. A conversation's `stateGen` is its chat log's generation, so a chat closed and opened again, or an engine that has started again, gives the page a new generation. A provider's, the window's and the update watcher's are random strings chosen when the owner is made.

**Events left out, and why.** `agent:commands`, `models:prices` and `agent:auth` carry no value the page applies in place: the first only makes the page read the skills again, the second is emitted once when a fetch ends, and the third is a short run of transitions from one sign-in flow. `agent:subagent` is emitted by the subagent service from values it built earlier and needs the same change as the queue; I would do it as a later step, after the page rule is proved on the first group. The change signals (`packages:changed`, `extensions:changed`, `workspace:changed`, `documents:recent`, `draft:changed`) carry no state, and `bus:changed` carries a counter that is already monotonic.

## What the page does

The rule lives in one place, a small module next to the sequencer. For each event name and owner, an owner being a session ID, a provider or the window, it keeps the last generation and number applied. An event with no `stateVer` is applied as today. An event with a different generation resets the record and is applied. An event whose number is higher than the record is applied and becomes the record. Any other event is ignored. When a session closes the page forgets its records. This module and its tests are Front End's, once the engine's fields are in.

## Where a resync's reread fits

A resync reads the state again through calls the page already makes: the sessions list, the provider states, the settings and the update state. Those calls return the same two fields, taken the same way: `Conversation.Info` for a live conversation, the stored state with its stored number for providers, and so on. The page runs a reread's result through the same rule as an event. This removes an assumption in the event-sequence plan, which said that applying an event that arrives during the reread is safe whichever comes first because events carry whole values. That was true of the values and false of the order. With the rule, a live event that is newer than the reread's snapshot wins, one that is older is ignored, and a reread whose snapshot is older than an event already applied is ignored. A conversation that is not live has no counter; its saved information has no `stateVer`, is applied, and the first live event after it has a number.

The unversioned `Info()` that internal code uses stays as it is and carries no number, so that nothing which compares information values changes. Only the emit helper and the calls the page uses to read state take a number.

## One emit path

As the chat events have one emit path (`TestChatEventsHaveOneEmitPath`), the versioned events get one: a source scan fails if any Go source other than the helper emits `agent:session`, `agent:queue`, `agent:options`, `agent:state`, `agent:account`, `settings:changed` or `update:state` by name. That keeps a later change from adding an emit that forgets the number.

## Tests

1. Many goroutines each change a conversation's queue and emit it, with the emits made in a different order from the changes by holding some back: the events sorted by number show the queue's length never going down, so each number describes the value it carries.
2. The same with `agent:session`: a goroutine that is held between reading and emitting delivers the older number last, and the numbers still match the states.
3. Provider state: two concurrent `state()` calls leave the stored state with the highest number, and the event with the highest number carries the stored state, whichever is emitted last.
4. The generation of a conversation changes when it is closed and opened again, and a number from the old generation is not compared with the new.
5. The calls the page uses to read state return numbers that are consistent with the events: a reread after an event is not lower than it.
6. The source scan described above, and a check that it finds what it looks for.
7. Everything above under the race detector, and a mutation check: taking the number outside the lock, or reading the value after taking it in a second acquisition, makes test 1 fail.

## DESIGN.md changes

- The bridge paragraph gets a sentence saying that state events carry a generation and a version taken from their owner's counter under the owner's lock, that the page ignores one older than the version it holds, and that a resync's reread is ordered by the same rule.
- The decisions table gets a row for the date.

## Questions for Architecture

1. Is it right to make the emit sites read the owner's current value at emit time, where today some emit a value built earlier? The events then say the latest state, which is what the page wants, and the older ones are dropped.
2. Is leaving `agent:subagent` for a later step acceptable, given that it needs the same change as the queue?

## Ruling

Architecture approved this shape on 29 October 2026 (message 591).

1. **An emit site reads the owner's current value at emit time.** Each event then says the latest state, with a version taken in the same lock acquisition as the copy. The version therefore orders snapshots, not changes: two emits of an unchanged value carry different versions, which is harmless, and any later snapshot is never older than an earlier one. A resync's reread takes its version the same way, so the page needs one rule for events and rereads alike, which is to apply a value only if its version is newer.
2. **A different generation always wins.** The page applies a value whose `stateGen` differs from the one it holds, whatever its `stateVer`, and adopts the new generation. That covers a restarted engine and a chat that was closed and opened again, and a test shows it.
3. **`agent:subagent` waits for a later step,** as proposed.
