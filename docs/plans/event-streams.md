# Recovering the Stream Events

## Status and scope

This is the proposal for commit 3 of [Sequenced Events with Resync](event-sequence.md), the part that recovers the stream events (`agent:update`, `agent:updates`, `agent:turn`, `agent:steer` and `terminal:output`). Dinah Engine Cleanup wrote it on 29 October 2026 at Architecture's request, before any code, and no code for it exists. The bridge (commit 1) and the four pending-request reads are written and in review. The proposal answers the design's open question first, then reports a finding that changes the design's premise.

## The open question: are a transcript append and its event under one lock?

No. The answer comes from reading the path an update takes from the agent to the page.

1. The agent's connection calls `Handler.Notify` in `internal/sessionhost/handler.go` for each `session/update`. For an ACP connection that call is made directly from the connection's reader goroutine (`internal/acp/conn.go`, the read loop), so one connection's updates are handled one at a time. I have not checked every call site of the Ollama model loop, so the design should not depend on a single goroutine making the calls.
2. `Notify` records the update with `RecordConversationUpdate`, which is `TranscriptCapture.RecordUpdate` and takes the shared transcript mutex. It then writes the chat log (`chatlog.Book.Update`, its own mutex), updates the journal, and for a replay queues the update under the conversation's mutex (`QueueIfReplaying`).
3. Last, it calls `Emit("agent:update", ...)`, which takes the bridge's lock and numbers the event.

Each of those locks is taken and released before the next. No lock covers both the recording and the numbering, so a reader that takes the transcript mutex can see an update whose event has not been numbered yet. Recording happens before emitting, which is the safe direction for a reader that wants nothing missed, and it is the wrong direction for a reader that needs to know exactly which events it already contains.

## The finding: an open chat has nothing to reload

The design says an open chat recovers by "reloading its transcript through the path it already uses when a chat is reopened", with a sequence point. Reading that path shows it cannot do this.

- `SessionLifecycle.openAt` returns the chat's `Info` immediately when the chat is already open in the engine (`internal/sessionhost/session_lifecycle.go`, the `l.conversation(id)` branch). It replays nothing. Replay happens only when the engine opens a session it does not have, by asking the agent to `session/load`.
- `SavedConversation` reads the transcript file from disk (`Continuations.Saved`, which calls `readTranscript`). The file is written when a prompt is recorded, when a turn ends, when a replay finishes and on a few other occasions (`History.FinishPromptLocked`, `FinishTurnLocked`, `FinishReplayLocked`). During a running turn it is stale by the whole turn so far.
- The in-memory copy that `RecordUpdate` builds keeps the person's and the agent's messages and the text of tool calls. It does not keep the agent's thinking, plans or the structure the page draws for a tool call. The page would draw a different chat from it than it draws from the live updates.
- The chat log is a Markdown file written for people to read.

So after a resync the engine has no source from which to hand the page the transcript it is showing, whatever sequence point it reports. Making the ordering atomic would be necessary and would not be sufficient.

## Proposal: a numbered log for each chat, and a call to read what was missed

Each open chat gets its own event log, kept by the engine, with numbers of its own.

**What is logged.** The events that make up a chat's transcript, in the form the page receives them: each update that reaches the page as `agent:update` or inside an `agent:updates` batch, and the chat's `agent:turn`, `agent:steer` and `agent:sent` events. Updates the engine diverts to other events (subagent updates, background work, option and command changes) are not logged, because the page does not draw them from this stream.

**How entries are numbered.** The log has a generation, a random name made when the engine creates the chat's conversation object, and a counter. Appending an entry takes the log's mutex, adds the next number to the counter and stores the entry. The mutex is a leaf: nothing else is acquired while it is held, and `Emit` is called after it is released. Every entry is therefore stored before its event is emitted, which is the one ordering the rest depends on.

**What the page receives.** Each of those events gains the generation and the entry's number (`gen`, `n`). An `agent:updates` batch gets the number of its first update, and its updates take consecutive numbers. The bridge's own sequence numbers and ring do not change, and the page keeps ignoring them.

**How the page recovers.** The page remembers, for each chat, the generation and the number of the last entry it applied.

- It applies an entry whose number is the next one, ignores one at or below the last (a repeat), and on one beyond the next it reads the missing ones from the engine first and applies them in order, then the new one. Two goroutines can emit for one chat in the opposite order to their numbers (a replay flush and a live update, for example). This rule makes that harmless, because the log already holds the earlier entry when the later one is seen.
- On `dinah:resync` it does the same for every chat it has open, asking for everything after its last number.

**The new call.** `ChatEvents(sessionId, gen, after)` is a query. Under the log's mutex it returns the entries after `after`, the current generation and the highest number. If the generation differs (the engine restarted, or the chat was closed and reopened), it returns that and no entries, and the page reopens the chat through the path it already has. If the log has dropped entries after `after`, it returns what it still has and says it is truncated, and the page marks the transcript where output was lost. The reply is consistent with itself because it is copied under one mutex, and nothing outside the log has to agree with it.

**Bounds.** Each log keeps at most a number of entries and a number of bytes, whichever is reached first, as options with defaults, and a total across all chats so that many open chats cannot grow without limit. I have not measured the size of updates in a long real chat, so the defaults are not set in this proposal. The first commit measures a long chat and proposes them with the numbers, as the bridge's bounds were proposed.

**What this removes from the design.** There is no transcript to reload and no sequence point to report, and no requirement that the log and the bridge's numbering be ordered together. A gap is detected by the page from the numbers it sees, and filled from a log that always holds the entry before its event is emitted.

## Alternatives I considered

1. **Reload from the saved copy, as the design says.** It is stale during a turn and lacks thinking and plan rows, as shown above. The page would show a different transcript after a resync than before it.
2. **Close the session and replay it from the agent.** It depends on each provider replaying a history faithfully, is unsafe in the middle of a turn, and costs a round trip to the agent for every chat on every resync.
3. **Make the bridge's ring larger.** The ring is shared by every event, including terminal output, and a resync means it was not large enough. A chat's own log gives each chat its own budget and an exact call.
4. **Mark the loss and recover nothing.** It is the honest fallback when a log is truncated, and the design already has it for notices. Using it for every stream would leave a hole in a transcript that the engine could have filled.

## Subagents and terminals

Subagent updates already carry a per-root `seq` and are kept in a bounded buffer (4,000 updates or 4 MiB for each subagent, in `internal/sessionhost/subagents.go`), and `SubagentUpdates(id)` reads them back. The page can use that existing number to detect a gap and that call to fill it. Front End has to confirm that the subagent view applies updates by `seq`.

Terminal output has no history in the engine at all. `pumpTerminal` reads from the shell and emits, and keeps nothing. The same idea applies, with the byte offset of each chunk as its number and a bounded scrollback per terminal, so that `TerminalOutput(id, from)` can return what the page missed. I propose that as its own commit after the chat logs, because it adds memory and a new call to a package that has neither. If a scrollback is not wanted, the page marks the terminal where output was lost.

## Tests

- Entries are stored before their events are emitted, checked by an emitter that fails the test if a subscriber sees an event whose entry is not yet readable.
- Concurrent emitters for one chat produce consecutive numbers with no repeat, and a reader never receives a partial prefix.
- `ChatEvents` returns the entries after a number, reports a changed generation, and reports truncation when the log has dropped entries, each with the size bounds set small.
- A replayed history takes consecutive numbers across batches, and a flush from two goroutines does not repeat a number.
- A page-side test in the browser suites, written with Front End, drops events, resyncs and checks that the transcript matches one that dropped nothing. The mutation check used for the bridge applies here too: with the append moved after the emit, the concurrency test must fail.

## Commits

1. The chat event log, the numbers on the events, `ChatEvents` and its tests, with the measurement of real update sizes (Engine). The page ignores the new fields until the second commit.
2. The page applies by number, fills gaps, recovers on resync and marks truncation (Front End, with Engine reviewing the contract).
3. Terminal scrollback and `TerminalOutput` (Engine, then Front End), if wanted.

## Questions for Architecture

1. Is a per-chat numbered log with a gap-filling call acceptable in place of the design's transcript reload, given the finding above?
2. Should `agent:sent` be in the numbered stream with `agent:turn` and `agent:steer`? I propose yes, because all three change what the transcript shows. `agent:file` stays a notice.
3. Should terminals get a scrollback, or should the page mark the loss?
4. This is the part of the work where a plausible mistake still passes the tests, as you said. Do you want the implementation commit written on Opus?

## Ruling

Architecture checked the finding against the code on 29 October 2026 (message 516). `SessionLifecycle.openAt` returns the open chat's `Info` and replays nothing, so there is no engine-side transcript to reload at a sequence point, and the design's reload is withdrawn for chats.

1. **The per-chat numbered log with a gap-filling call is approved** in place of the reload. A gap is found from the numbers the page sees and filled from a log that holds an entry before its event is emitted, which needs no ordering between the log and the bridge's numbers.
2. **`agent:sent` joins `agent:turn` and `agent:steer`** in the numbered stream. `agent:file` stays a notice.
3. **Terminals mark the loss for now.** Scrollback adds memory and a call to a package that has neither, and the person has not asked for it. Commit 3 is deferred until commits 1 and 2 have shipped and Paul has seen how often a terminal loses output.
4. **Commit 1 is written on Opus**, and Architecture reviews it on Opus. The ordering of append and emit is the one place here where a plausible mistake still passes the tests.

Conditions on commit 1:

- **One emit path.** The events are emitted from about twenty-five places in `turn_io.go`, `steering_io.go`, `bus_delivery.go` and the shell's update batching. Every one of them goes through a single function that appends to the chat's log and then emits, and no other code emits these events. A test that scans the sources fails when `agent:update`, `agent:updates`, `agent:turn`, `agent:steer` or `agent:sent` is emitted anywhere else.
- **The log is a leaf.** Its mutex is released before `Emit` is called, nothing else is acquired while it is held, and the order is append, then release, then emit. The mutation check from the bridge applies: with the append moved after the emit, a concurrency test must fail.
- **The numbers are the log's.** The page-side rule is stated in the proposal (apply the next number, ignore a repeat, fill a gap from `ChatEvents` first), and `ChatEvents` is added to the page-method table as a query.
- **Bounds come with figures.** The measurement of a long real chat is posted first, with the proposed per-chat and total bounds, before the bounds are set.
- **Replay is numbered too.** Updates from a `session/load` replay take numbers in the same log, and a flush from two goroutines never repeats one.
- **Closing and reopening a chat changes its generation.** Front End confirms that the page's reopen path on a changed generation does not draw the transcript twice.

## Commit 1 as written

Engine wrote commit 1 on 29 October 2026. Four details differ from the proposal's wording, and the page's commit relies on them.

1. **Replayed updates are numbered when they are queued, not when the batch is sent.** `QueueIfReplaying` and `QueueReplay` number each update in the chat's log under the same hold of the conversation's mutex that queues it, so the numbers follow the order the agent sent the updates, whichever goroutine later sends the batch. Numbering at the flush would let two flushes take numbers in the opposite order to their content.
2. **An `agent:updates` batch carries `ns`, the number of each update, not only the first.** The numbers in one batch rise but need not be consecutive, because an event of another kind for the same chat can be numbered between two queued updates. The page applies each update with its own number.
3. **Every event's data is a map with `gen` and `n` added to a copy.** The log keeps the data without them, as `{n, name, data}`, and `ChatEvents` returns those entries. An `agent:updates` batch is kept as one `agent:update` entry for each update. An event for a session with no open chat is sent as before, without a number, and the page applies it as it does today.
4. **A chat's `Info` carries `streamGen` and `streamHead`**, read from its log, so a page knows the generation of a chat it has had no event from yet, such as one opened in a second window.

The bounds are 32,768 entries or 16 MiB for each chat and 64 MiB across all open chats, as Architecture decided (message 530) from the saved transcripts. Each trim is logged with the chat's entries and bytes and the total. An entry larger than the per-chat byte bound is numbered but not kept, and a page that missed it is told the log is truncated. The total is a running count that may overshoot briefly while a trim is under way.
