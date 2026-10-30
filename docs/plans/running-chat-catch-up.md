# Showing a Running Chat's Turn to a Window That Opens Late

This is the shape of the next engine item in [Group Chat Work After the Cleanup](group-chat.md), the fourth in its list of things the cleanup showed, written by Dinah Engine Cleanup on 29 October 2026 at Architecture's request, before any code. A window opened on a chat that is in the middle of a turn draws the saved copy and then follows the chat from its log's head, so it misses the turn so far. The chat's event log, which the cleanup added, can supply it. The DESIGN.md change is listed at the end and made in the commit with this document. No code for it exists.

## What happens today

A chat's saved copy is the transcript file, which Dinah writes at fixed points: when a chat is opened, when a replay ends, when a prompt has been recorded at the start of a turn, when a turn finishes, and when a handoff is consumed or begun (`History` in `internal/sessionhost/history.go`). What a turn says while it runs is kept in the history in memory and reaches the file only when the turn finishes.

`SavedConversation(id)` reads the file. The page's restore path draws it, reconnects to the agent behind it, and, for a chat that is already live in the engine, gets back its information with the head of the chat's event log (`streamHead`) and follows the log from there (`chat-sequence.js`, `info`). Everything the turn has said since its start is therefore on neither side of that line. It is not in the file, and it is before the head. This happens whenever a page opens on an engine that kept running: a page that reloads in the middle of a turn, and the daemon the plan heads for, in which the engine outlives its windows.

The log does have it. Every update of the turn went into the chat's log in the form the page draws, with thinking and plan rows, which the saved copy never has, because the page does not draw anything from the log that it did not draw live. The log is bounded (32,768 entries or 16 MiB for a chat), so the turn so far is there unless it was longer than that.

## The shape

**1. The file says which log entry it is consistent with.** When the history writes the file, it also records the log's highest number at that moment: `streamThrough`, kept with the saved state and returned beside the messages. Everything in the file is from entries up to that number, and everything after it is not in the file. For a turn that is running, the last write was when its prompt was recorded, so the number is the log's head at the start of the turn.

**2. `SavedConversation` returns it for a live chat.** The view gains `streamGen` and `streamThrough`, set when the chat is live in the engine and its log is the one the number refers to, and absent otherwise. A chat that is not live has no log, and its view is as it is now.

**3. The page starts the log from that number.** Having drawn the saved messages, the page takes the generation and number as the position of the chat's sequencer and asks `ChatEvents(id, gen, through)` for what follows. It applies those entries as it applies any, by the numbers, and carries on live. The entries are the turn so far in the form the page draws, so the result is what a page that had watched the turn would show. This is Front End's change, and it replaces the rule that a chat takes its start from the head its information reports, for a chat drawn from a saved copy. A log that no longer has entries after `through` says it is truncated, and the page marks the transcript where output was lost, as it already does.

## The one thing that has to be exact: the pair

The number is only useful if the file and the number describe the same moment. A file that includes an update the log has not got yet would be drawn and then applied again when its entry arrives, so one update would show twice. A file that lacks an update the log already has would lose it, if the number were past it.

Today a live update is recorded in the history under the capture's lock and, after that lock is released, appended to the log. A reader that takes the file's content and the log's head can therefore see the history one update ahead of the log. The fix is to append the update to the log inside the same critical section that records it, so that the history and the log never differ by an update to anyone who holds the lock. The log's own mutex is a leaf, which may be taken under any lock, so this adds no ordering problem. Only the append moves. The emit still follows the release, as the chat events have always done, so no event is emitted under a lock.

With that, the history's commit points read the log's head under the lock they already hold when they write the file, and the pair is exact by construction.

## What it does not do

- It does not change what the saved copy holds. The thinking and plan rows stay out of it, and the replay-versus-saved-copy difference, the fifth of the cleanup's list, is a separate item.
- It does not help a chat that is not live. Its page still draws the saved copy and the agent's replay takes over, as now.
- A turn that outgrew the log is partly lost and the page says so; the bounds are Architecture's and unchanged.

## Tests

1. A turn in progress: the updates of a turn are logged and recorded while a reader takes the saved view over and over; for every view, the saved messages plus the logged entries after its number show each update exactly once, with none missing and none repeated.
2. Without the fix, the same test fails: with the append left after the critical section, a reader sees an update recorded and not yet logged.
3. The number is the log's head at the last write, for each commit point: opening, the end of a replay, the recording of a prompt, the end of a turn, and a handoff.
4. A chat that is not live returns a view without a generation or number; one that is live returns the generation of its log, and a closed and reopened chat returns a new one.
5. Through the page's calls: a page that opens on a live chat mid-turn gets the turn so far from `ChatEvents` after the saved copy, and the transcript equals the one of a page that watched from the start. This test is written with Front End.
6. Under the race detector, and a mutation check for each of the two: the number taken before the write, and the append outside the critical section.

## DESIGN.md changes

- The bridge paragraph gains a sentence saying that a window opened on a live chat draws the saved copy and then the log's entries after the number the copy is consistent with, so the turn in progress is shown.
- The decisions table gets a row for the date.

## Questions for Architecture

1. Is it right to move only the log append into the critical section that records the update? It makes the pair exact, and nothing is emitted under a lock.
2. Should the page start from the saved copy and the log only for a chat that is live in the engine, leaving every other chat as it is now?

## Ruling

Architecture approved this shape on 29 October 2026 (message 608), with the first answer widened.

1. **Every logged event that the saved history also records is appended in the same critical section that records it, not only updates.** A prompt is recorded in the history when its turn starts, and it reaches the page as `agent:sent`; a turn's end is recorded and reaches the page as `agent:turn`. If either were appended to the log after the history's lock was released, a saved copy written in between could hold the prompt or the turn's end while its log entry sat above the recorded head, and the page would draw it twice. Events the history does not record, such as a steering message's state changes, may stay as they are. The log's mutex is a leaf, so nothing else changes in the lock order, and every emit still follows the release. The test that takes saved view after saved view during a turn includes a prompt and a turn's end as well as updates, and the mutation check covers each of them.
2. **Only a chat live in the engine gets the number,** as proposed. A chat that is not open has no log, and it is drawn from its saved copy as today.

## What was built

Built on 29 October 2026 (Dinah Engine Cleanup). Reading the code for the ruling's first answer showed that the three kinds of event are not in the same position, so the fix is different for each.

- **Updates** were recorded under the transcript mutex and logged after it was released, in the goroutine that handles the agent's notifications, while the turn's own goroutine writes the saved copy at its end. They are now logged in the critical section that records them (`TranscriptCapture.RecordUpdateLogged`), and the handler emits the event after the section.
- **A turn's end** was recorded and written, and its event logged later, once the turn's outcome (including whether it was steered) was known. The turn loop now records how the turn ended, and `CommitTurn` appends the turn's event and writes the saved copy in one critical section, so the copy that holds the turn's end is written with a number that includes its event. The write is therefore a few operations later than it was, after the bus is looked at for more messages.
- **A prompt** needed no change. Every site that logs a prompt's `agent:sent` does so before it records the prompt, in the same goroutine that writes the copy, so a copy written after the record always has the event below its number. The steering sites log before they record for the same reason. A test shows that the check sees the opposite order, so a change that reversed one would be caught only if it is exercised; the order is a rule of those sites and is stated in their code.
- **The number** is kept by the history, which writes the file through one function that notes the log's head. `SavedConversation` returns it with the log's generation for a chat that is live and has written its copy since it was opened, and not for one that is replaying its history.

Tests: a reader takes the saved view over and over while a turn's prompt, updates and end are recorded and logged by two goroutines, and every event is in the copy exactly when its number is at or below the recorded one. Two mutation checks fail it: the turn's event appended after the write, and an update appended after the critical section (the second through a hook that lets a commit get in between). A test also covers the number at each commit point, the absence of a number for a chat still replaying, and the window's call for a live chat after a turn.
