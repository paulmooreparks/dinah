# Reopening a Chat With session/resume

This is the shape of the item Paul approved on 29 October 2026 (recorded in [Group Chat Work After the Cleanup](group-chat.md) under "Opening a Chat Without Replaying Its History"), written by Dinah Engine Cleanup at Architecture's request, before any code. A long chat takes minutes to reopen because `session/load` makes the agent replay the whole conversation, about 108,000 parts for one chat. `session/resume` reopens a session without returning the previous messages. The shape says which providers advertise it, what the replay does that resume skips, how a copy that may lack a cut-off turn is handled, what the page is told, and the answers to the two questions Front End asked. The DESIGN.md change is listed at the end and made in the commit with this document. No code exists.

## What the agents advertise

Checked on 29 October 2026 from each agent's `initialize` response, which Dinah reads as `agentCapabilities.sessionCapabilities`.

| Provider | Advertises `resume` | How it was checked |
| --- | --- | --- |
| Claude, adapter 0.87.0 | Yes, with `fork`, `close`, `list`, `delete` and `subagents` | The adapter was started and initialized with Dinah's own client capabilities. |
| Devin | No. It has `loadSession`, `list`, `delete` and `additionalDirectories`. | The same, with `devin acp`. |
| Codex | No | Dinah's own adapter makes this response (`internal/codexagent`) and sends no `resume`. |
| Ollama | No | The same, `internal/ollamaagent`. |

So only Claude gets the fast reopen now. Codex's adapter reopens a thread with `thread/resume` and then produces the replay itself from the thread's items, so a resume for Codex would be a change in Dinah's own adapter, to be measured when Codex is available (its limit resets on 4 November 2026). Ollama keeps its conversations in Dinah's own file, so its replay is cheap. Neither is part of this item. The decision is made from the response at each reopen, never assumed, and the response type gains `Resume` beside `List`, `Delete` and `Close`.

Measured on the same day against the Claude adapter, with a small session: `session/resume` with the session's ID, folder and servers answers with `configOptions` and `modes`, as `session/load` does, and sends one `available_commands_update` and no previous messages. A prompt to the resumed session was answered in a way that showed the agent had the earlier conversation (it said it had not followed an instruction given earlier), so the agent restores its own context from its own records, as the plan says. `session/resume` of an ID the agent does not have answers "Resource not found". The saved session files these runs left were removed.

## What the replay does that resume skips

Reading `SessionLifecycle.Open`, a reopen today does the following around `session/load`, and each item needs an answer for a resume.

1. **It marks the chat as replaying and restoring** and starts a transcript in replay mode, which keeps the old saved copy aside and records the replayed rows into a new one. With resume the chat is created restoring and not replaying, and the history is opened in a mode that keeps the saved copy as it is (a new mode of `History`, since opening without a replay today starts an empty copy). The saved copy is the chat's transcript from then on, and the first live turn appends to it.
2. **It numbers the replayed updates in the chat's event log** and flushes them to the page in batches. With resume there are none. The chat is a new conversation with a new log generation and an empty log, so what the page is given is the saved copy and the pair (generation, 0), which is the pair it already starts from for a live chat (`streamGen` and `streamThrough` of `SavedConversation`).
3. **It compares the replay with the saved copy and may replace the copy** (`FinishTranscriptReplay`), which is where the replay disagreement warnings came from. With resume there is no replay and no comparison, so the warning cannot arise, and the copy is never replaced behind the person's back.
4. **It settles a user message the replay ended on and ends the replay** (`SettleReplayedPrompt`, `FlushReplay`, `EndReplay`). Resume has no replay to settle.
5. **It re-announces the chat's subagents and then ends those left running** (`DropSubagentIndex`, `DisconnectSubagents`). The subagent tree is not part of the saved copy, so a resumed chat shows no subagents from earlier turns until new work creates some. This is a loss against a full load, and Load Full History restores it.
6. **It carries the agent's thinking and plan rows**, which the saved copy never has. A resumed chat does not show them for earlier turns, as a chat after a restart does not today before its replay is drawn. Load Full History restores what the agent sends.
7. **It brings the title and update time from `session_info_update`.** Dinah already has the title in its settings and reads the agent's list only for a chat without one, so nothing changes.
8. **It returns the agent's config options**, which Dinah restores the chat's settings from (`RestoreOptions`). `session/resume` returns them too, so the same code runs on the result.

Everything else in `Open` stays: the applet channel, the bus identity and its server, the chat log, joining the bus, the substitutions and the information returned.

## When resume is used and when load is

Resume is used only when all of these hold, and otherwise the reopen is the `session/load` it is today.

- The agent's response advertises `resume` (read at this reopen).
- Dinah has a saved copy of the chat that is whole: the transcript file exists, is not deleted, belongs to this provider and workspace, is marked complete, and has no capture error.
- The person did not ask for the full history.

**A copy that may lack a cut-off turn is not resumed.** The saved copy is written when a turn starts and when it ends, and what a turn says while it runs reaches the file only at its end. If Dinah stopped in the middle of a turn, the file has the prompt, is marked not complete, and lacks what the agent said and did. The agent's own records have it. A chat in that state fails the test above and is reopened with `session/load`, whose replay brings the missing turn in and replaces the copy, as it does today, with the existing warning that the last turn was cut off. The same goes for a copy with a capture error or one that was never complete. A crash after the turn ended but before the file was written is the same case, because the file would not be marked complete; a crash after the write leaves nothing missing.

**What a resume cannot see** is a change to the session made outside Dinah, such as the person continuing the same Claude session from the command line. The replay's comparison used to notice that (the `OutsideHistory` finding). With resume it does not, and Load Full History is how the person brings it in. I would say so in the command's description.

**A resume the agent refuses** falls back once. If `session/resume` fails with something other than "not found" (the method is not available, say), the reopen goes on with `session/load`. A "not found" is the same answer `session/load` would get and is reported as any failed reopen is, with the saved copy still shown.

## What the page is told

- `Open` returns the chat's information as now, with a new field `resumed`, true when the reopen used `session/resume`. Nothing is replaying, so the chat is ready as soon as its options are restored, and a message waiting for it is delivered at once instead of being announced as arriving.
- `SavedConversation` for the open chat returns `streamGen` and `streamThrough` with the saved messages, as it does for any live chat. For a resumed chat `streamThrough` is 0 and every live event is numbered from 1, so the page draws the copy and starts its sequencer at (generation, 0), which is the page half of the running-chat catch-up that Front End has already built. A resumed chat needs no new page mechanism for drawing.
- A chat restored with the layout already draws its saved copy before it reconnects. With resume the reconnect finishes in about the time it takes to start the agent, and the page keeps the copy it drew, since there is no replay to take its place.

## Answers to the page's two questions

1. **A resumed reopen gets the pair (generation, 0).** A reopen creates a new conversation as any open does, so its log has a new generation and no entries. The saved copy covers everything before the log began, so `streamThrough` is 0, and the code agrees: the history records the log's head when it writes the copy, and opening the copy writes it with the head at 0. The engine's new generation tells the page it is a different chat state from any it held before.
2. **Load Full History is a new generation, by closing and reopening.** The protocol documents `session/load` for starting a conversation's replay but not for a session that is already active, and the adapters do not say what they do, so Dinah does not call it on an open session. The command closes the conversation first (the agent's `session/close` where it advertises it, and always Dinah's own removal of the conversation), then reopens it with `session/load`. The page treats it as it treats a chat that was closed and opened again: it clears the drawn copy, drops the old generation's queued events, and draws the replay as it arrives. The command refuses while the chat is working or switching, as closing would otherwise cut a turn.

The command is a window method, `LoadFullHistory(id)`, which returns the chat's information as `Open` does. It is also available for a chat that Dinah opened with `session/load` already, where it replays again, which is harmless.

## Tests

1. A fake agent that advertises resume and a whole saved copy is reopened with `session/resume` and not `session/load`; the chat is not replaying, its copy keeps its messages and its completeness, its options are restored from the result, its log has a new generation with `streamThrough` 0, and `Open` says it was resumed.
2. Each reason for load: no `resume` advertised, no saved copy, a copy not marked complete, a copy with a capture error, a deleted copy, a copy of another provider or workspace. Each reopens with `session/load` and replays as before.
3. A resume the agent refuses with an error other than "not found" falls back to `session/load`; "not found" is reported as a failed reopen and the copy stays.
4. A cut-off turn: a copy written at the start of a turn and not ended is reopened with `session/load`, and the replay replaces the copy with the turn in it.
5. Load Full History on a resumed chat closes the conversation and reopens it with `session/load`, has a new generation, replays into a fresh copy, and refuses while the chat is working.
6. A message waiting for a resumed chat is delivered at once and not announced as arriving.
7. The decision is made from the response at each reopen: the same chat is resumed with an agent that advertises it and loaded with one that does not.
8. Mutation checks: with the completeness test removed, test 4 fails; with the capability not read, test 7 fails.
9. A live measurement with Claude, run by hand and not in the suite, shows a long enough chat reopens in the time it takes to start the agent. It is recorded in the plan with the numbers.

## DESIGN.md changes

- The paragraph on restoring chats after a restart says that a chat whose agent advertises `resume` and whose saved copy is whole is reopened with `session/resume`, with no replay, and that anything else, and the Load Full History command, uses `session/load`.
- The decisions table gets a row for the date.

## Questions for Architecture

1. Is it right that a chat is resumed only when its saved copy is complete and without a capture error, so a crash-cut chat takes the slow path once and then has a whole copy again?
2. Is losing the subagent tree and the thinking rows of earlier turns on a resumed reopen acceptable, given that Load Full History restores them?

## Ruling

Architecture approved this shape on 29 October 2026 (message 644). It rests on documented behaviour: the Agent Client Protocol defines `session/resume` as resuming a session without returning previous messages, behind `sessionCapabilities.resume`, which is read from each agent's initialize response at every reopen rather than assumed per provider.

1. **Only a whole saved copy is resumed.** A copy that may lack a turn cut off by a crash takes the slow load once, which fills in the turn and replaces the copy, and later reopens are fast again.
2. **A resumed reopen does not show earlier turns' thinking rows or the subagent tree** until the person runs Load Full History. Paul accepted the loss of thinking and plan rows when he approved the fast reopen, and the subagent tree is the same kind of thing.
3. **A resume the agent answers with "not found" is treated as load is today when it fails.** The session no longer exists in the agent, so falling back to a load that would fail the same way only doubles the wait; the chat reports that it could not be reopened, as it does now. Any other refusal falls back to load once, as proposed.

## What was built

Built on 29 October 2026 (Dinah Engine Cleanup), the engine half, as approved. Reopening a chat reads the agent's `sessionCapabilities.resume` each time. When it is there, the person did not ask for the full history, and the saved copy is whole, the chat is created restoring and not replaying, the copy is kept with its messages and flags and written again (so its log position is the new log's 0), the replay steps are skipped, and the options are restored from the resume result. `Open` returns `resumed` true for such a chat. A resume refused for any reason but "not found" falls back to `session/load` once, and "not found" fails the reopen with the copy untouched.

`LoadFullHistory(workspace, id)` is a window method. It refuses while the chat is restoring, switching or working. It asks the agent to `session/close` the session where that is advertised, and leaves the chat as it is if the agent refuses, since a load is never sent to a session the agent may still hold active. It then removes the conversation and reopens it with `session/load`, which is a new log generation.

Three things the plan did not say. The wholeness check and the opening of the copy are one step under the transcript mutex, so a copy that changes in between cannot be resumed by mistake, and a copy that fails the check at that moment takes the load. The DESIGN.md change is the decisions-table row only, since the restore behaviour has no paragraph of its own to amend. And a reopen of a crash-cut copy keeps its cut-off note until the replay brings something the copy lacks, which is the case the test covers; a replay that brings nothing new leaves the note, as `FinishReplayLocked` already did.

Not done: the live Claude timing measurement the plan lists as test 9, because it needs a long chat and a person to read the numbers. Codex and any resume for it wait for 4 November 2026.
