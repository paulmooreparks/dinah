# Opening a saved chat: a deadline, and no held call

On 21 November 2026 the whole window stopped answering. One chat's agent never answered the request to load its session. `OpenSessionAt` waited for that answer with no limit, and the page's call waited with it. The page talks to the engine over HTTP/1.1, with one request for each call and one stream for events (`internal/bridge`), and a browser runs only a small number of requests to one host at a time. Five opens that never returned, with the event stream, used all of them, so no other call could be made.

The cause of that day's silence was a limit on the length of a line in the agent connection's reader, and it is repaired: a line of any length is read, and a reader that ends fails the calls that wait on it. This note records the two rules that keep the same shape of fault from stopping the window for any other cause.

## Rule 1: a reopen has a deadline

The request that reopens a chat is `session/load`, or `session/resume` for an agent that advertises it. The engine waits for its answer for `ReopenLoadLimit` at most, which is 90 seconds (`boundedLoad` in `internal/sessionhost/session_lifecycle.go`). At the limit the open fails with a plain error, and the open then ends as every failed open ends: the chat's channel is revoked, the chat is taken out of the runtime and the page is told it is closed.

### Why 90 seconds

The limit has to be longer than any healthy load. The longest load in the logs of that day replayed a history of 56 MiB and was answered in well under a minute. The reopen watch (`cmd/dinah-desktop/reopen_watch.go`) writes its line and saves the goroutine stacks at one minute, so a load that reaches the limit has left that evidence first.

The limit also has to be shorter than the wake scheduler's patience. A message that wakes a closed chat asks for a connect, and the scheduler forgets that request after `bushost.ConnectPatience`, two minutes. A failure that arrives after that is no longer treated as the answer to the request: the chat's team is not told that its lead could not be opened, and members would wait for a relay that never comes. The page is given five seconds to open the chat before the engine does, so a load may take 115 seconds at most, and 90 leaves room for the steps around it. `TestTheReopenLimitFitsTheWakePatience` fails if the two values stop fitting.

The value is a constant in code. Nobody can choose it well without reading logs, so it is no setting. A chat whose healthy load is longer than the limit can never be loaded, which the log shows as the same chat failing at the limit each time; the remedy is to raise the limit and `ConnectPatience` together.

### What happens to the load that was given up

The agent's process is left running, because one process serves every chat of its provider, and one slow chat is no reason to end the others' turns. The request is not withdrawn either, because the Agent Client Protocol documents no request that cancels a load. (`session/cancel` is documented for a prompt turn.)

The request therefore stays open on the connection. JSON-RPC matches an answer to its request by the request's id, so a late answer reaches that request and nothing else. The engine logs it and discards its result. Until the answer arrives, the chat is marked as owed one, and the mark does two things.

- A second open of the chat on the same connection fails at once with an error that says so, and sends nothing to the agent. Two loads of one session at the same time would each replay the conversation, and the protocol does not say what an agent does with the second.
- The `session/update` notifications the agent still sends for that session are dropped (`Handler.Notify`). The protocol documents that an agent replays the conversation as such notifications before it answers a load, so these are the replay of the load nobody waits for. They reach neither the saved copy nor the page.

When the late answer is a success the agent holds the session loaded. Where it advertises `session/close`, the engine closes that session, as it closes a chat's session before loading it again (`reopen`). The mark ends with the answer. It also ends when the connection does, since a connection that ends fails its waiting requests.

A `session/resume` that reaches the limit is followed by no `session/load`. A refused resume falls back to a load; a resume with no answer is no refusal, and its request is still open.

### What the person and the bus see

The chat's window shows the note it shows for any failed open, with Try Again. For this failure the note's title is "*Agent* did not finish opening this session." and its text is the engine's sentence, without the words "*Agent* reported", since the agent reported nothing. The saved history stays in the window.

Messages that wait for the chat keep waiting. A wake's delivery attempt (`internal/sessionhost/wake_attempts.go`) ends with the error and is recorded as `failed`, the stage every failed open leaves, so the Notifications card "Messages are waiting for ..." appears with Retry Waiting Message. An attempt no longer stays at `opening` because its agent is silent.

### What is still unbounded

The steps of an open after the load are not bounded by this rule. They are the requests that restore the chat's settings (`session/set_config_option`, in `Options.Restore`), which new chats use too. An agent that answers the load and then never answers one of those still leaves that one chat at Loading. With rule 2 it no longer costs the page a connection.

## Rule 2: opening a chat holds no page call

The page opens a saved chat with `BeginChatOpen(token, method, args)` (`cmd/dinah-desktop/chat_opens.go`). It gives the opening method it would have called and that method's arguments: `OpenSessionAt`, `OpenWaitingSession`, `OpenSessionWith`, `ReopenSessionWith`, `LoadFullHistory` or `RetryWaitingSession`. The call starts the method in the engine and returns at once. The outcome is sent as the event `chat:open-result`, on the one stream every other event uses, with the token, and with the chat's information or the error and its detail.

In the page, `chatOpen` in `helpers.js` wraps this as a promise that resolves with what the method returns and rejects with what a failed call of it rejects with. The code that opens a chat (`session-lifecycle.js`) changed only in which function it calls. Everything the window shows while a chat opens, and where it shows it, is decided by the same code as before, at the same moments: Loading stays until the promise settles.

The opening methods themselves are unchanged and still return when the agent has answered. Their other callers want that: the task host, the service chat port, the bus wake and the tests.

### The token

The page chooses the token, a random value, before it calls. The event can arrive before the answer to `BeginChatOpen` does, since the two come over different connections, and the page must already know which open the event is for.

A token the engine already knows starts nothing. The answer is `opening`, or `done` with the outcome. The engine keeps the outcomes of the last 256 opens that ended for this.

### A page that reconnects or reloads

The bridge numbers its events and replays the ones a reconnecting stream missed, which the browser asks for with the `Last-Event-ID` header that the EventSource interface documents. When the bridge can no longer replay them it sends `dinah:resync`. On a resync the page asks again for each open it still waits for, with the same token, and takes the outcome from the answer when the open has ended. If that question fails, the open is shown as failed, with Try Again, which finds the chat open if it is.

A page that was reloaded has lost its tokens. Its windows open their chats again with new ones, and an open of a chat that is still loading joins the load in flight: `openExact` keeps one open for each chat and gives every caller its result. The same holds for several windows, the bus wake and the page opening one chat at the same time.

### Exit

A page call that changes something counts as unfinished work when the window is asked to close. `BeginChatOpen` is listed as a query, and the open it starts counts itself as unfinished work under the opening method's name until it ends, so the exit guard sees what it saw before.

## Checks

- `cmd/dinah-desktop/chat_opens_test.go`: a reopen past its limit leaves the chat closed, the message waiting and the attempt retriable; a retry beside the unanswered load is refused and sends nothing; the late replay and the late answer change nothing; an open started from the page returns at once, reports by event and shares one load; over real HTTP with six connections, six opens that never finish leave another call answered.
- `cmd/dinah-desktop/frontend/e2e/chat-open-by-event.js`: six chats that never finish opening hold no call; a chat whose open ends at the limit shows its error in place and can be tried again; a lost outcome is learned after a resync.
- The other browser suites describe an open by the opening method, and the suites' stub (`e2e/lib/page.js`) answers `BeginChatOpen` by running that method and sending the event, so they all pass through the event path.
