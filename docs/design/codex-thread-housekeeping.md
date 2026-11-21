# Keeping Dinah's threads tidy in Codex

Codex keeps one store of threads for every program that uses it, and the Codex desktop app lists that store. Dinah drives Codex through `codex app-server`, so every Dinah chat with Codex is a thread in the same list. On 21 November 2026 the list showed what Dinah had left there. A chat was shown by its first message, which was often a line Dinah itself had written, because Dinah never named a thread. The earlier stages of chats that had been switched to another program or recovered were still listed as live threads, and so were the chats in Dinah's Recycle Bin. The judge's one-question sessions were never ended, and a test left a thread behind on each run.

This note describes the five changes that followed, the schema text each one rests on, and what cannot be done with documented features.

## The rule the changes follow

Dinah never relies on undocumented behaviour of an external program. Every call below is in the protocol schema that Codex 0.162.0 prints with `codex app-server generate-json-schema`, and the quotations are from that schema. Dinah does not read the text of a Codex error to decide what to do, and it does not read Codex's files.

None of these calls is needed for a chat to work. A call that fails is written to the log and the person's action goes on.

## 1. A judge's session is closed

A judgement call starts a thread with `ThreadStartParams.ephemeral` set to true (`internal/codexagent/bare.go`). The schema gives the parameter as an optional boolean, and it describes the matching field of the returned thread, `Thread.ephemeral`, in these words:

> Whether the thread is ephemeral and should not be materialized on disk.

The session was ended with `thread/delete`, which Codex refused, and the refusal was discarded. The session then stayed in the adapter's table of sessions and the connection stayed subscribed to the thread.

A thread that is kept in memory only has nothing stored to delete, so such a session is now ended by closing it. The adapter sends `thread/unsubscribe` (`ThreadUnsubscribeParams`, with the one required field `threadId`; the answer's `status` is one of `notLoaded`, `notSubscribed` and `unsubscribed`). The adapter removes an ephemeral session from its table whatever Codex answers, because the thread cannot be resumed either way. A failure to end a judge's session is logged.

The adapter says which of its sessions are ephemeral (`SessionEphemeral`), from what it asked for at `thread/start` and from `Thread.ephemeral` in the answer. A judge session that is not ephemeral is deleted as before.

## 2. A thread has its chat's title

The schema has a request `thread/name/set` whose parameters, `ThreadSetNameParams`, are `threadId` and `name`, both required strings. It describes `Thread.name` in these words:

> Optional user-facing thread title.

Dinah gives the thread the chat's title, which is the name the person gave the chat or else the title a judge wrote for it. A chat with neither is shown in Dinah by Codex's own title, so there is nothing to send.

The title is sent at these moments:

- The person renames the chat in Dinah, whether the chat is open or closed.
- A judge has written the chat's title.
- The chat has been loaded again. `thread/resume` returns the thread with its `name`, the adapter remembers it, and it sends the title only when the two differ.
- The chat moves to a new thread, by a switch to Codex, by continuing from saved history, or by recovery. The continuation gives the new chat the old chat's title, which names the new thread.

A new chat has no title when its thread is started. Its thread is named when the first title arrives, from the judge or from the person.

Dinah's title is the authority. A thread that somebody renames in the Codex app gets Dinah's title back the next time its chat is loaded.

The schema does not say what `thread/name/set` does with an empty name, so Dinah never sends one. When the person clears a chat's name, the thread is given the judge's title if the chat has one, and it keeps the name it has otherwise.

## 3. Threads Dinah no longer uses are archived

The schema has `thread/archive` and `thread/unarchive`, each with the one required parameter `threadId`. It documents one effect of archiving, on `thread/list`, in the description of `ThreadListParams.archived`:

> Optional archived filter; when set to true, only archived threads are returned. If false or null, only non-archived threads are returned.

Dinah archives a thread in two cases:

- The chat continues in a new thread. When a switch, a continuation or a recovery is committed, the earlier stage's thread is archived.
- The chat goes to the Recycle Bin. Each thread of the chat is archived once the chat is in the bin.

Dinah records each thread it archived (`archivedThreads` in `settings.json`). Before a recorded chat is loaded again, for any reason, the thread is taken out of the archive with `thread/unarchive`, and the record is removed when Codex confirms. Restoring a chat from the Recycle Bin does the same at once when Codex is running. If the unarchive fails, the failure is logged, the load is asked for all the same, and the record stays so that the next load asks again.

Archiving never starts Codex. When Codex is not running, the chat is recorded as waiting (`threadsToArchive`), and the thread is archived the next time a Codex chat opens. A chat that is in use again by then is dropped from the wait.

A request to archive that Codex does not answer within ten seconds may still be carried out later. Dinah records such a thread as archived, so that it is taken out of the archive before its chat is loaded.

The earlier stage of a switched chat keeps its session loaded for a while after the switch, as it did before this change. The schema does not say whether a loaded thread can be archived. Dinah asks at the switch, and if Codex refuses, it asks once more when that session is ended.

Final deletion from the Recycle Bin is unchanged. It uses `thread/delete` (`ThreadDeleteParams`, with `threadId`), behind the ownership check for spawned descendants.

### 3b. Archiving closed chats, as a trial

Settings › Agents has a new choice under Codex, **Archive Closed Chats in Codex**, which is off unless the person turns it on (`codexArchiveClosed` in `settings.json`). With it on, the thread of a Codex chat is archived when the chat's session is ended. The thread is taken out of the archive before the chat is loaded again, by the same record as above, so a chat that a waiting message opens is opened as any other. Turning the choice off leaves archived threads as they are until their chats are next loaded.

A session is ended at one of two moments.

- **When the person closes the chat's last window, if the chat is idle.** This was added on 22 November 2026 so that the trial can be judged in minutes. Dinah waits five seconds (`windowCloseSettleDefault`), because the page reports its windows at every change and a page that is loading again restores them one by one, and a chat whose window is back by then is left alone. Then it closes the session with `session/close`, which the Codex adapter carries out as `thread/unsubscribe`, and archives the thread.
- **After the chat has been closed with no window for four hours** (`sessionhost.ClosedSessions`). This is the path every provider's chats take, and it is the one a chat takes when it was not idle at the moment its window closed.

A chat is idle for the first case when every one of these holds, which are the rules the four-hour path already follows (`ClosedSessions.EndNow`, `Activity.ClosedIdleLocked` and `BeginEndLocked`) with the bus looked at as well:

- No turn is running in it, which covers a question or an approval the agent waits for, since the agent asks those inside a turn.
- No work of the agent's goes on after a turn, and no subagent of the chat is running.
- It is not open in the background for a message or for its team.
- Nothing holds its runtime, as a task and a service chat do (`HoldRuntime`).
- It is not being configured, switched to another program, loaded or ended.
- No message that would wake it waits on the bus (`bus.Waking`), and no delivery of such a message is under way.

A chat that fails any of these keeps its session and is ended by the four-hour path. Dinah does not look again when the chat becomes idle, so a chat that was working when its window closed is archived four hours after it stops, at the earliest. If the messages waiting cannot be read, the chat keeps its session.

The work runs apart from the page's report of its windows, so closing a window never waits for Codex. A session that Codex does not close stays loaded, and a thread that cannot be archived stays as it is. Both are written to the log and nothing else changes.

### What the trial costs

Dinah keeps a closed chat's session loaded for four hours for a reason: the person, or a message, brings the chat back at once, without a load (`docs/plans/group-chats.md`, decision 11). With this choice on, a Codex chat that the person closes while it is idle gives that up. Opening it again takes out of the archive first and then loads the thread, which for a long conversation takes seconds where it took none. A message for the chat that arrives after the window was closed pays the same, since it opens the chat in the background by a load.

That is the trade the choice makes: a tidy list in Codex against a quick reopen. It is a trial switch, off unless the person turns it on, and with it off nothing here happens and closed chats keep their sessions as before.

Whether the Codex desktop app hides archived threads from its list is not documented. The schema documents the filter of `thread/list` and nothing about the app. The trial is how the owner finds out. With the choice on, close the window of a Codex chat that is not working, wait about ten seconds, and see whether the Codex app still lists it; the log has the line "archived the thread of chat ..., which was closed" when Codex accepted the request. A look that needs no setting is to move a Codex chat to the Recycle Bin.

Chats that are closed because Dinah itself is closed are not archived. Their sessions end with the Codex process, and Dinah makes no calls for them on its way out: shutdown ends the five-second waits that have not finished, and starts no new one.

## 4. The live test deletes its thread

`internal/codexagent/bare_live_test.go` asks the installed Codex one question in an ordinary thread and one in a bare thread. It now deletes the ordinary thread with `thread/delete` when the answer is in, and closes the bare one.

## 5. Advice when a Codex chat cannot be opened

Codex gives a thread to one program at a time. When another program holds a thread, Codex refuses the resume with a message the person cannot act on. No documented call tells Dinah that this is the reason, and Dinah does not read the message to guess.

The note for a chat that could not be opened therefore keeps Codex's words as they were and adds one sentence below them, for every failed open of a Codex chat:

> Codex gives a conversation to one program at a time, so if this conversation is open in another program that uses Codex, such as the Codex app or an editor, closing it there and pressing Try Again may free it.

The advice is a field of the provider's module (`OpenFailureAdvice`), so a provider that sets none shows none. The page shows it only where the note offers Try Again. It is left out when the session no longer exists at the provider, because trying again cannot help there.

The window of a chat that could not be opened already shows Dinah's saved copy of the conversation, so reading the thread's history with `thread/read` for that window was left out.

## How a provider offers these

Each capability is optional. A provider's connection offers one by having the method, and shared code asks by interface (`cmd/dinah-desktop/agent_threads.go`), as it does for the receipt of a new session:

| Capability | Method on the connection | Codex call |
| --- | --- | --- |
| Name a thread | `SetSessionTitle` | `thread/name/set` |
| Archive a thread | `ArchiveSession` | `thread/archive` |
| Take a thread out of the archive | `UnarchiveSession` | `thread/unarchive` |
| Say which sessions are in memory only | `SessionEphemeral` | none |

The provider's module says the rest: whether it archives threads at all, whether the person's settings archive closed chats, and the advice for a failed open. A provider with none of this is left alone.

## What documented features cannot do

- **Hide a live chat from the Codex app.** A thread that Dinah is using is an ordinary stored thread. The schema has no field that keeps a thread out of another program's list while it is in use. Archiving is the one documented way to take a thread out of the default `thread/list`, and the schema does not say that an archived thread can run a turn, so Dinah archives only threads it is not using.
- **Learn whether the Codex app hides archived threads.** The schema describes the protocol. What the app lists is the app's own behaviour.
- **See that another program holds a thread.** No request or field reports which program has a thread open.
- **Release another program's hold on a thread.** `thread/unsubscribe` ends this connection's own subscription. Nothing ends another program's.
- **Clear a thread's name.** `ThreadSetNameParams.name` is a required string, and the schema does not say what an empty one does.
