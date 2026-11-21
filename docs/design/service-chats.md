# Service Chats

A product built from this source can give its own service a plain port for working in chats. The owner decided on 21 November 2026 that the base offers this port beside the task host. The task host is unchanged, and `docs/design/task-host-binding.md` remains its contract.

## Why there are two ports

The task host reviews the exact input of every operation before a run starts, sends a turn only to a chat that existed at the review, and has the person confirm each run. That suits work whose steps are known in advance. It cannot serve a service that builds each prompt from the answer before it, or that opens chats as it goes, because neither the later prompts nor the later chats exist at the time of a review.

The port described here has no review and no confirmation of a run. A service that uses it is responsible for asking the person before it starts work. The port keeps the guarantees that protect a chat and its agent, which are listed under Guarantees below. It adds no way around the agent's permission policy.

A product chooses one port per service. A service registered with `services.RegisterWindowNamed` receives a task client. A service registered with `services.RegisterChatWindow` receives the port of this note. A service name is registered once, whichever kind it is.

## Registration and life

The package `internal/servicechats` has the port, its errors and the window object that admits and joins the calls. The registration is in `internal/services`.

```go
services.RegisterChatWindow("sample", "Sample", []string{"sample-page"},
    func(port servicechats.Port) (services.Service, services.Participant, error) {
        s := newSample(port)
        return services.Object(s), s, nil
    })
```

The window builds these services at start, after the task services, in the order of their names. The page calls them through `CallService`, as it calls every other service, and no page method was added. A port acts under the name its service was registered with, and the service cannot choose another name.

The window shuts the port down with the task host, in this order.

1. The task host and the chat port close their admission. From then on every call of the port is refused with `ErrClosed`.
2. Closing the admission cancels the context of every call that is running. A running turn asks its agent to stop, and returns within four seconds whether the agent stops or not.
3. Each service's `Shutdown` is called, with the five seconds the task services have.
4. The window waits, within the same five seconds, until every call of the port has returned. Calls that are still running then are written to the log.
5. The window lets go of the chats it kept loaded for the services.

A call runs on its caller's goroutine. A turn starts one goroutine that waits for the owned turn, and the call returns only after that goroutine has finished. The turn runner every turn has is joined by the window when it closes the chats' turns, as it is for the person's turns.

## What identifies a workspace

A service gives a workspace by the full path of its folder. The port accepts the folder when exactly one saved workspace has it as the folder its chats run in. It compares paths as the rest of the window does, which ignores letter case on Windows and resolves no links. A folder that no saved workspace has, a folder that two saved workspaces share, a relative path and a folder that is missing are all refused with `ErrNotWorkspace`. Every workspace the window has open is a saved workspace, so an open workspace is accepted too.

## The methods

All methods take a context first. A context that has ended is refused before anything is done.

| Method | What it does |
| --- | --- |
| `OpenChat` | Opens a new chat in a workspace with an agent program, and gives it a title. |
| `TakeTurn` | Sends a prompt to a chat the service opened and waits for the end of the turn. |
| `TakeTurnInExistingChat` | Takes the same turn in any chat of the person's, by its identity. |
| `CancelTurn` | Stops the turn this service's port started in a chat. |
| `Model`, `UseModel` | Read the model of a chat the service opened, and set it with a function that puts the earlier one back. |
| `RenameChat`, `ReleaseChat` | Rename a chat the service opened, and end the service's use of it. |
| `SkillAvailable`, `SkillPrompt` | Say whether a skill can be used with a program in a workspace, and prepare text that starts with a skill's name. |
| `Programs`, `CheckProgram` | List the agent programs the product offers, and say whether one can be reached. |
| `PublishChange` | Sends the event `service:<name>:changed` with a workspace and a key. |
| `DescribeAction`, `ActionDigest`, `RunAction`, `CancelAction` | Describe, run and cancel a registered applet action for the service. |

### Chats the service opened and chats the person chose

`OpenChat` makes an ordinary chat. It is opened by the function New Chat uses, it is in the Chats list at once, and the person can open it, read it, write in it, rename it and delete it. The window saves which chats each service opened, in the folder `service-chats` of its state. `TakeTurn`, `Model`, `UseModel`, `RenameChat` and `ReleaseChat` act only on those chats and refuse any other with `ErrNotOpened`. One service's port does not act on the chats another service opened. A chat that was continued with another agent program is still the same chat for the port.

`TakeTurnInExistingChat` is the one method for a chat the service did not open. It is meant for a chat the person chose for the service's work. It makes the same checks and gives the same refusals as `TakeTurn`. The separate name keeps a service from writing into a chat by accident, since an identity that is not one of its own is refused by `TakeTurn`.

The port has no chats that are kept off the Chats list. The window's one way of keeping a session off the lists (`sessionhost.RegisterNotChat`) is for sessions another program ran. A working chat that is missing from the lists would also be a chat the person cannot find when its agent asks for a permission. A chat of a service is therefore listed from the moment it is opened. A product that needs an unlisted session can still use `RegisterNotChat` itself, with its own records.

### Turns

A turn is the exclusive, owned turn the task host takes (`sessionhost.TurnIO.TakeTurn`). The port calls it through `TakeTurnStoppable`, which was added for it and which is the same turn with one more way to stop it. `TakeTurn` itself behaves as before. The prompt is built as the person's own message is built, so a prompt that starts with a skill's name is prepared for the agent program, and the transcript shows the prompt under the label the service gave.

A chat is refused at once, and nothing is sent, queued or steered, in these cases.

- The chat is working, which includes a turn of the person's, work the agent left in the background and running subagents. The error is `ErrBusy`.
- The chat is switching agent programs. The error is `ErrSwitching`.
- The chat is in the Recycle Bin or deleted. The error is `ErrRemoved`.
- The service released the chat. The error is `ErrReleased`.
- The window is closing. The error is `ErrClosed`.

Every other failure of a turn is one of two kinds. A failure that wraps `ErrNotSent` happened before the prompt was handed to the agent's connection, so the turn may be tried again. A failure that wraps `ErrUnknownOutcome` happened after that point, so the agent may or may not have received the prompt. The port never sends such a prompt again, and a service must not either without the person's word.

When the context ends, when `CancelTurn` is called or when the window closes, the agent is asked to stop its turn with the protocol's `session/cancel`. A turn the agent then ends has the stop reason `cancelled` and the text written so far. Its error is the context's when the context ended, `ErrClosed` when the window closed, and nil after `CancelTurn`. The port waits two seconds for the agent to end a turn it was asked to stop, and the owned turn then waits up to two more. A turn the agent has not ended by then fails with an error that wraps both `ErrUnknownOutcome` and the context's error, and the chat stays working until the agent settles. A context that ends before the prompt was handed over stops the turn with nothing sent.

`CancelTurn` stops only a turn that this service's port started. It reports false, and does nothing, for a chat in which the person or anything else is working.

### Models

`Model` reads the chat's option of the category `model`, which is where an agent that follows the protocol offers its models. `UseModel` sets it through the chat's options service with the exclusive, recorded setter the task host uses (`sessionhost.Options.SetRecorded`). A chat that is working is refused with `ErrBusy`, and a value the agent does not offer with `ErrModel`. The function it returns puts the earlier model back with `RestoreRecorded`. It refuses with `ErrModelChanged` when the chat's settings were changed in between, so the person's later choice is never undone. The change is kept in memory only, so a model that was set is not put back after a restart. The model a service sets becomes the chat's saved model, as it does when the person sets one.

### Skills

`SkillPrompt` is the preparation a chat's own messages get (`skillPromptFor` in `skills.go`). An agent program that reads skills itself gets the text as it is. Another program gets the skill's instructions with the text. Text that starts with no known skill's name comes back unchanged. A turn prepares its prompt in the same way, so a service needs `SkillPrompt` only to see or keep the prepared text. `SkillAvailable` reports a skill that is in the list the chat's picker shows for the program, or that the preparation finds instructions for.

### Programs

`Programs` lists the agent programs of the product's profile. `CheckProgram` uses the check the Settings window runs (`AgentCheck`). It starts the program when it is installed and not running, and it sends no prompt. A program the product does not offer is reported without any check.

### Change notices

`PublishChange` sends `service:<name>:changed` with the service's registered name, a workspace folder and a key of the service's own. The event has no content of a record. A page that receives it reads the record again from its service. A service has either this port or a task client, so the event of one service always has one shape.

### Applet actions

The applet broker already runs an action for an owner and records the invocation, and the task backend's use of it is short. The port reuses it. An invocation of a service is recorded with the owner kind `service` and the service's name, which the broker now accepts beside `chat`, `applet` and `task`. `RunAction` needs a deadline, checks the digest the service gives, and cancels the invocation when the context ends. A caller may supply the invocation's identity, in which case running it again with the same fields reads the saved invocation and repeats nothing.

An action that requires the person's approval of every invocation is refused with `ErrNeedsPerson`. The task host asks the person about such an action through its own question records, and this port has no question of its own. An action with other effects runs without a question, as it does for an applet's page. The person's question about an action belongs to chats, where an agent decides to run it, and the service's code is the product's own.

A failed invocation is an unknown outcome unless the broker recorded a failure of an action that may be repeated, or recorded nothing at all.

## Guarantees

- A busy chat is refused and nothing is sent. The refusal is decided under the chat's own lock, by the admission every turn passes.
- A prompt with an unknown outcome is never sent again by the port.
- A cancelled context stops the turn, and so does the window's shutdown.
- A chat a service opened is an ordinary record that the person can open, read and delete.
- The agent's permission requests during a turn of the port appear in the chat as they do for any turn, and wait for the person. The port answers none.
- Nothing in the port changes or bypasses the agent's permission policy, the file-access policy or the chat's approval setting.
- After the window's shutdown no call of the port is running, within the budget described above.

## Limits

- The port has no review of a run and no confirmation by the person. The product's service asks the person before it starts work.
- The port keeps no receipts. A service that must know after a restart whether a turn was sent saves that itself before and after the call.
- `OpenChat` cannot be interrupted once the agent has been asked for the chat. A failure after that point wraps `ErrUnknownOutcome`, and the returned chat has the identity when the agent gave one.
- `ReleaseChat` does not close the chat with its agent. The window closes it later by its ordinary rule for chats nobody has open, as it does for the task host.
- The record of the chats a service opened is never pruned. It grows by one small entry for each chat.
- A turn is counted among the unfinished work the person is told of when they close the window, as a task operation is.

## What the implementation rests on

Everything the port does goes through functions of this source, which are given above and in `cmd/dinah-desktop/service_chats.go`. Two facts about the agent protocol are used, and both are in its specification. The first is that `session/cancel` ends a turn with the stop reason `cancelled`. The second is that a session's configuration option of the category `model` selects its model. The point at which a failed turn changes from not sent to unknown is the base's own choice. It is the last check before the prompt is handed to the connection, which is earlier than the moment the request is written, so some failures are reported as unknown although nothing left the program.

## Tests

`internal/servicechats` tests the admission, the refusal of a second turn, cancellation, shutdown and the count of goroutines with an invented backend. `internal/services` tests the registration and the order of start and shutdown. `cmd/dinah-desktop/service_chats_test.go` tests the window's side against an invented agent: an ordinary chat in a saved workspace, the refusals with nothing sent, the unknown outcome that is not sent again, cancellation, a permission request shown in the chat and answered by the person, the model set and put back, release, skills, the change notice, applet actions and the window's shutdown.
