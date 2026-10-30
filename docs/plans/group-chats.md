# Group Chats Made of Chats

Paul said on 27 October 2026 that the bus and its group chats are hard to use and to understand. He sees names such as `pudl-1` and `studio-15` and cannot tell how to start a group conversation with the chat windows he has open, or with chats that are closed. This plan records why, what he decided, and the design that follows. A mockup of the picker and the group window comes before any of it is built.

## Why It Is Hard Today

A survey of the code on 27 October found five causes.

- A chat's name on the bus is its workspace folder's name in lower case with the first free number after it (`internal/bus/bus.go`). The chat's title never reaches the bus, and renaming a chat changes only Dinah's settings. The Bus pane shows these names everywhere, and its data has no titles at all. Only the separate web page that `dinah serve` starts shows a chat's title beside its name.
- A room's members are workspaces and the person. Naming a chat when making a room adds that chat's whole workspace, and the room window suggests only workspaces. A group of three particular chats cannot be made.
- No chat window offers to start a group. A room is made from a form in the Bus pane where the members are typed as text.
- A chat that is not open after a restart is not on the bus, so it cannot be named as a recipient, and nothing opens a chat because a message arrived for it. DESIGN.md left that as open question 5.
- Two mechanisms reach another chat. An agent asking another chat finds it by title and puts the question straight into it, and only the answer comes back over the bus. Bus messages and rooms use the folder-based names. Nothing in the window connects the two kinds of name.

## Rulings

Paul ruled on 27 October 2026.

1. **Group chats are made of chats,** and of the person, not of workspaces. This reverses the decision of 23 October 2026 that rooms are channels between workspaces.
2. **A message for a closed chat opens it.** Dinah opens the chat in the background, without a window, to deliver the message, accepting that each such opening costs an agent turn.
3. **Asking another chat moves onto the bus,** so there is one way for chats to reach each other.
4. **The plan and a mockup come first.**

## Design

### A chat is known by its title

Every chat keeps the bus identity it has today, the `window-…` ID that survives a restart, and that ID becomes an internal key that nobody reads. Wherever a person or an agent sees a chat, they see its title and its workspace: in the Bus pane, in a group's member list, on the card of a delivered message, in a group's conversation, and in what agents are told, such as "from PUDL Development (workspace pudl)". A chat without a title yet is shown as "Untitled Chat in" its workspace. Renaming a chat renames it everywhere at once, because nothing stores the title a second time.

The folder-based handle stays inside the bus as an alias for one release, so a handle that an agent saved in its notes still reaches the chat, and is then removed. The person's own handle, `paul`, is unchanged, as the profile plan already records.

An agent names a chat by its title, and adds the workspace when two chats share a title. When a name matches more than one chat, the tool refuses and lists the candidates with their workspaces, as `dinah.chat/find@1` does today. The bus tools that list or address sessions (`sessions`, `whoami`, `send`, `reply`) take and return chats in this form.

### Group chats

What the code calls a room is shown as a group chat. A group's members are chats and the person, and the person is always a member. The person names the group when making it, and a group made without a name takes one from its members' titles.

A message in a group goes to every member chat. The rule for waking stays as it is: a message that mentions a chat, with an @ and the chat's title, wakes that chat, and every other member reads the message at the start of its next turn. That keeps a busy group from starting a turn in every chat at every message, and it answers DESIGN.md's open question 2 for now. The person's own messages follow the same rule, so the person decides whom to wake by mentioning them.

Existing rooms are converted once. A room's workspace member becomes the chats of that workspace that the bus knows at the time, and the room keeps its messages. Workspace membership then no longer exists, so the bus has one model, not two.

### Starting a group

A chat window's Actions menu gains Start a Group Chat… and Add to a Group Chat…. The Bus pane's New Room becomes New Group Chat…. Each opens the same picker.

The picker lists every chat Dinah knows, open or closed, grouped by workspace, with each chat's title, its provider, whether it is open, working or closed, and when it was last active. A filter box narrows the list as the person types. The chat the picker was opened from is already ticked. The person ticks the chats to include, names the group or leaves the name to be made, and chooses Start. The group's window opens with its members, its conversation and a box for the person's message.

### The group window

The window shows the members as a row of chips, one for each chat, with its state, a menu to open that chat's window or remove it from the group, and Add Chats…, which opens the picker. Below them is the conversation, with each message under its sender's title, and below that the box where the person writes, with @ to mention a member. Leaving or deleting the group is in the window's menu.

### Closed chats

When a message is delivered to a closed chat that it wakes, either because the message mentions the chat or because it is addressed to the chat directly, Dinah opens the chat in the background as it opens a saved chat today, without a window, and delivers the message as the chat's next prompt. The chat then appears in the chat lists as Open in Background until the person opens its window or it has been idle for thirty minutes. Dinah opens at most three chats in the background at once and queues the rest, so a message to a large group cannot start a crowd of agents. A message that does not wake a closed member waits for it, as messages do today.

### One way to reach a chat

Asking another chat becomes a bus message that expects an answer. The asking chat sends the question over the bus to the chat it found by title. It is delivered like any other message, so it can open a closed chat, and the answer comes back as it does today. The applet actions `dinah.chat/find@1`, `list@2` and `ask@2` keep their names and their contract for the agents that use them, and only how they deliver changes. `list@2` lists closed chats as well, marked as closed.

### The Bus pane

The pane shows four sections: the person's inbox, the group chats, every chat by workspace with its state, and the trusted links. The list of who is on the bus is replaced by the list of chats. Trusted links stay between workspaces, because trust concerns what an agent may do in a folder, not which chat it is.

## Decisions From the Mockup

The mockup is in `docs/mockups/group-chats/`, with screenshots in its `shots` folder. Paul accepted it on 27 October 2026, and with it eight answers to questions that building it raised.

1. Add to a Group Chat… opens a submenu of the groups the chat is not in, with New Group Chat… at its foot.
2. A mention is one unit. Choosing a chat from the @ list in the message box inserts it whole, and the message keeps a list of the chats it mentions. An agent gives that list when it sends, because a title with spaces cannot be found reliably in the text.
3. The person's @ list offers only the group's members, with Add a Chat… after them. An agent's message that mentions a chat outside the group is refused, with the reason.
4. A chat opened in the background has a state of its own, Open in Background, with its own glyph, so the person can see which chats are spending turns without a window.
5. A group message that mentions the person is also put in the person's Inbox.
6. @everyone stays. It first woke only the members that were open. On 30 October 2026 Paul decided that it wakes every member and opens the closed ones in the background, because that is what the word says (docs/plans/group-attention.md).
7. A chat must have a title before it joins a group. The picker asks for one in place, since two rows reading "Untitled Chat in studio" cannot be told apart.
8. The person is always a member, so a group has no Leave. Its menu has Delete Group… instead, and the group's messages are kept.

Paul accepted the two proposals that had been left open on 28 October 2026.

9. A chat opened in the background closes itself thirty minutes after its last turn ends, unless the person has opened its window by then.
10. At most three chats are open in the background at once. The rest wait in a queue and open as slots free, so their messages are delayed and never dropped.

Neither limit stops a chat joining a group or taking part in one.

Paul decided on 28 October 2026 that closing the session after thirty minutes idle would be too extreme, and that the session should end "after the chat has been closed for some amount of time rather than just idle, and for longer than 30 minutes".

11. When a chat has been closed for four hours without a break, Dinah ends its session with the agent. Dinah Development chose four hours as the default, and it can be adjusted. The chats this covers are those that keep a loaded session while closed: a chat open in the background that closed itself for being idle, and a chat whose window the person closed, which Dinah keeps open with no window. A chat that has a window, is open in the background, runs a turn or has the agent's work running after a turn is not closed, so opening it before the four hours are up cancels its timer and the next closing starts a new one. The session is ended with the protocol's `session/close` where the agent advertises it; an agent without it is stopped instead, but only when every session it has loaded is due and it can load them again, and otherwise its sessions stay loaded. A chat whose session has ended is closed as a saved chat is, and opening it again, by the person or for a message that wakes it, loads it as opening a saved chat does.

## Questions Left Open

None are left open.

## Order of Work

1. The mockup of the picker, the group window and the Bus pane, for Paul to mark up.
2. Titles everywhere, with the handle kept as an alias.
3. Groups made of chats, with the conversion of existing rooms, and the picker and the group window. The Bus pane becomes the Group Chats pane in this step, as Paul decided on 28 October 2026, since by then it shows group chats and chats rather than rooms of workspaces.
4. Opening closed chats in the background.
5. Asking another chat over the bus.

## Progress

Step 2 is built. The bus keeps each chat's title in its session's row, and the window writes it there when the chat joins the bus, when its agent titles it and when the person renames it. The window also writes each workspace's name, which the bus calls the workspace's label. Every heading, list and address reads the title from that one place. Agents send to a chat by its title, adding the workspace in parentheses when two chats share it, and a title alone that more than one chat has is refused with the candidates and their workspaces. The folder-based handle is shown nowhere and is still accepted as an address, and as an @ mention in a room. That alias is to be removed in the release that delivers step 3. A chat in the window is no longer offered the bus's `rename` tool, because its title is the one the window shows; a session outside the window uses `rename` to set its title.

Step 3 is built. A group's members are chats, by their sessions on the bus, and the person, who is always a member and cannot be taken out. Workspace membership is gone. When the bus opens a file from before, it converts each room once: a workspace member becomes the chats of that workspace the bus knows, which are the open ones and the window's closed ones, the room takes its old name as its title, the person becomes a member, and the messages stay. Converting a file already converted changes nothing. A group has a key, which its window's address uses, and a name, made from its chats' titles when none is given. A chat that has never been open since the bus began is recorded as known but closed when the picker puts it in a group, under the session it keeps when it opens.

A group's message goes to every member chat but its sender and wakes only the chats it mentions. The mentions are a list kept with the message. Agents give titles in the new `mentions` field of `send` and `reply`, in the same "Title (workspace)" form as an address, and the window gives the sessions of the members chosen from the @ list. Text with @ and a title mentions nobody. Everyone wakes the members open at that moment and none that are closed. A mention of the person also puts the message in the person's inbox, and an agent's message reaches the inbox only then; the group's other messages count as unread in its window until the window shows them. An agent's mention of a chat outside the group is refused, with the group's members in the reason, and so is a chat outside the group writing to it. A closed member's messages wait for it, and a mention of it rings when it next opens. `room_create` takes member chats by title, `room_join` and `room_leave` act on the calling chat, and the tools' descriptions and the agents' instructions speak of group chats made of chats.

The Bus pane is the Group Chats pane, in its title and the View menu, with the inbox, the group chats, every chat by workspace with its state, and the trusted links, and New Group Chat… and New Message… on its toolbar. The picker and the group's window are built as the mockup shows them, each a window with an address, and the picker keeps the chats ticked in its address. A chat window's Actions menu has Start a Group Chat… and Add to a Group Chat…. The list of who is on the bus is gone from the pane, and with it the button that took a workspace with no chat off the bus. The picker's note on a closed chat said that the group's messages wait for it until it opens, until step 4 changed it.

The folder-based handle is removed. It is no longer an address or a mention, a new session is given none, and nothing a person or an agent reads shows one. The sessions table keeps its name column, which now holds a new session's ID, because dropping a column from a file that an older process may still have open is not worth its risk. The bus's own page lists group chats and starts one from chats given by title, without mentions, so a message written there wakes nobody.

Step 4 is built, on 28 October 2026, together with a fix for a fault Paul reported that day. After a restart, several chat windows came back showing the prompt to reconnect and stayed there, so the group messages that mentioned them were never delivered. The window did not know those chats were waiting for anything, and their reconnecting one after another could wait indefinitely behind one chat whose agent did not reply.

The bus watcher now looks, every second, for messages waiting that would wake a chat: a message sent to the chat itself, a group message that mentions it, or a reply it waits for. A message for a workspace does not count, since no single chat is asked to answer it, and neither does everyone, which wakes only the members open when it is sent. When such a message waits for a chat that is not connected to its agent, Dinah connects the chat through the same path the prompt's Reconnect Now button takes. A chat whose window is open, such as one restored at startup, reconnects in its window with its saved copy in sight while the agent replays, and a failure is reported above the prompt as it is when the person presses the button. A chat with no window is opened the same way without one, and a failure is reported in a notice. Either way the message is then delivered as any message is, and a message whose chat failed to connect keeps waiting until a newer one asks for the chat again. Since the messages wait on the bus, a restart pokes their chats on the watcher's first look. The page tells the window which chats have windows, and nothing is connected until it has, so a window still being restored is never taken for a closed chat. The restored chats still reconnect by themselves one at a time, but one that has not finished within a minute no longer delays the next.

A chat opened with no window is Open in Background, shown as the positive badge with PUDL's eye-off glyph in the Group Chats pane, the picker, a group's member chips and the chat lists. Opening its window makes it an ordinary open chat. At most three chats are open in the background at once, counting those being opened, and the rest wait in the order their messages came. A chat open in the background closes itself thirty minutes after its last turn ends: it leaves the bus, so its messages wait for it, it is shown as Closed, and its group membership is unchanged. Dinah keeps its session with the agent, so a later message or the person opening its window brings it back without the agent loading it a second time; the protocol does not say what loading a session that is already loaded does. Decision 11 later limited how long the session is kept. The picker's note on a closed chat now says that a message that mentions it opens it in the background.

The docked pane's tab still read Bus in a layout saved before the pane was renamed, because the window kept the last title of each open window in its saved layout. A window of which there is only one now always takes its kind's title.

Step 5 is built, on 28 October 2026, so asking another chat now travels over the bus and there is one way for chats to reach each other. `dinah.chat/ask@2` sends its question as a bus message of its own kind, a question, from the asking chat's session to the asked chat's. It is a message to the chat itself, so step 4's watcher wakes the chat for it: a closed chat opens in the background, within the cap of three, and a window restored at the prompt to reconnect is reconnected in place. The bus never hands a question over as an ordinary message. The invocation that asked waits until the chat is connected, takes the question, and runs it as a turn whose words are the answer, steered into a busy chat's turn where its provider allows it, as before. The answer, an error, a cancellation or an expiry reaches the asking chat as the completion report it already received. The action's name, input and output schemas, its digest's rule and its contract of one invocation and a reply that arrives later are unchanged; the digest itself changes, as it does whenever the implementation does, and it now also covers the delivery over the bus and the waking code. `ask@1` takes the same path, and only how its result is read differs.

A question that loses its invocation is withdrawn, so it cannot open a chat for nobody: the invocation withdraws it when it ends without taking it, and the watcher withdraws any that no invocation of the running window waits for, such as those left by a window that stopped. A chat that fails to open ends the question with the agent's reason. Questions are exempt from the hop limit, since the existing guard against chats asking each other in a loop and the rule that a chat answering a question asks nobody already stop a chain, and the bus refuses a reply to a question.

In the asked chat the question is the card of a message from the asking chat under its title, as any message the bus delivers is, and in the asking chat the answer is the card of a reply from the asked chat, as before. `list@2` and `find@1` list closed chats too, with a state of open, open-in-background or closed for each, and open chats first. The closed ones are those of the window's workspaces of which Dinah keeps a saved copy, less hidden, deleted and continued chats, titled by the person's name for them or by the bus, and an untitled chat is listed as the bus shows it, "Untitled Chat in" its workspace. The instructions agents receive describe one mechanism and keep the guidance they rely on: invoke once, do not poll or sleep, and the reply arrives by itself.

Two things are left as they were. A person's @ mention of a chat in a chat's own box still requires the mentioned chat to be connected, since the page's list offers open chats, and asking a closed chat is the agent's to do. A chat opened in the background for a question whose asker then cancels stays open until its idle close, thirty minutes later.

Decision 11 is built, on 28 October 2026. The four hours are one constant, `closedSessionEnd` in `closed_sessions.go`, and the bus watcher keeps each chat's clock at every round once the page has said which chats have windows. Dinah's adapters for Codex and Ollama now advertise and answer `session/close`. For Codex the adapter interrupts any turn, unsubscribes from the thread with app-server's `thread/unsubscribe`, after which app-server unloads the thread once its grace period passes, and `session/load` resumes the thread with `thread/resume` as before. For Ollama it cancels any turn, lets the conversation go, since its record is saved after every turn, and stops the applet servers no other conversation uses; what the person allowed in it is kept for the rest of the run. The Claude adapter Dinah runs advertises `session/close` itself, and for Devin Dinah uses it when the running Devin advertises it. A session the agent fails to close stays loaded, the chat is shown as closed, and Dinah tries again after another four hours.

## The Group Window Matches a Chat

Paul reported on 28 October 2026 that a group chat lacked much of what a chat has, such as a box that grows as he types, and his screenshots showed group messages printed as raw Markdown. The group window had its own small box and drew each message as plain text, so every improvement to a chat had to be copied to it by hand, and none had been.

The chat's message renderer and the behaviour of its box now live in two shared files, and the chat, a group and a thread all use them. `chat-text.js` draws a message from its Markdown, makes the result safe through `safe-html.js`, turns addresses into link chips, hands d2 and Mermaid blocks to the diagram engine, and builds the message card with Edit or Quote, Copy, and the foot a long message repeats them in. `composer.js` holds the box's growth, the send key from Settings › Chat, Up and Down through the box's history with the caret's visual row deciding when they apply, a box a window builds in code with the chat's markup and classes, and the saved drafts of every box, the chat's included. The chat's box remains in `index.html`, because the chat's toolbar, skill form and attachments are built around it, and it has the same classes. A group configures the shared pieces with its own @ list of members, everyone and Add a Chat…; a mention chosen from that list is part of what the box holds, so a draft, a message brought back with Up and one put back with Edit all keep their mentions as units, as decision 2 requires.

Each feature of the chat's box and messages was decided for a group as follows.

- The group's box grows with its text up to the chat's limit.
- Enter or Ctrl+Enter sends, as Settings › Chat says, and Send's tooltip says which key it is.
- Up from the box's first visual row and Down from its last bring back the person's own messages in that group, with their mentions. The history is the group's messages from the person, which the bus already keeps, so it outlasts a restart.
- A draft is kept per group while its window is closed, as a chat's is, and both survive a restart, which Paul decided on 28 October 2026. When Up has brought back an earlier message, the draft it set aside is what is kept, so Up and then Down return to the draft after a restart too. The draft keeps its mentions as units with the list of chats they mention. Sending drops the draft, and so does deleting the group in its window. The section below on drafts across a restart says where they are kept.
- Messages are drawn from Markdown with the chat's sanitiser, link chips and diagrams, and mentions are marked after the Markdown is drawn, outside code and links.
- Copy, Quote into the group's box and the long-message foot work as in a chat. The person's own messages offer Edit instead of Quote, as a chat's do, and Edit puts back the text with its mentions.
- Each message's time is in the regional format and is written again if the format arrives late.
- The list stays at its end while the reader is there and keeps its place otherwise, and messages already drawn are kept rather than drawn again. The chat's Jump to the Latest button and its saved scroll position are not shared, because they belong to the chat's log and its pinned last message.
- Pasting or dropping images and files was not offered at first, since the person has no workspace for a bus attachment to live in. The plan in [group-attachments.md](group-attachments.md) added a store for the person's files and the delivery of pictures as images, and the group's box now takes them with the chat's chips.
- Skills from / are not offered, since a skill runs in a chat's agent with that chat's workspace, and a group has neither.
- The provider, mode and model pickers, Steer, Stop, the document in context, usage and the queue are not offered, since a group has no agent of its own.

A thread window uses the same shared box and renderer, so its replies are Markdown, its box grows and recalls the person's replies, and it keeps a draft per thread. Group delivery, waking and opening chats in the background are unchanged.

## Drafts Across a Restart

Paul asked on 28 October 2026 for the unsent draft of a chat and of a group to outlast a restart, so that the box shows it again and Up then Down returns to it as before. Until then Dinah left typed text out of its saved window state on purpose, and a draft lasted only while Dinah ran.

Drafts are now kept in the saved UI state, `ui-state.json` in Dinah's home folder (`~/.dinah`), beside the window layout and the Editor's open drafts. Nothing is written into a workspace folder, since a draft can hold private text and a workspace may be a shared repository. Each draft has its own entry: `dinah-draft:chat:` and the chat's session, `dinah-draft:chat:new` for a new chat with no session yet, `dinah-draft:group:` and the group's name, and `dinah-draft:thread:` and the thread's number. A draft is saved as it changes, through the UI state's usual write a moment after each change and its flush when the window is hidden or closed. A box reads its saved draft only once the saved state has been loaded and the layout restored, and it saves nothing before then, so an empty box made early never writes over the draft it is about to show; this is the discipline the scroll positions follow (`ui-state.js`, `readSaved`). A box that already holds text when its saved draft arrives keeps that text.

A chat's draft is its text, the mentions chosen for it and the workspace documents attached to it. Files attached from the computer or pasted, such as screenshots, are not kept across a restart: the window holds their contents only in memory, at up to 25 MB each, and the whole saved UI state is bounded at 32 MiB, so a single large file could stop the layout from being saved. They still stay with the draft while Dinah runs. A chat continued with another provider takes its draft with it. Removing a chat drops its draft, and hiding one keeps it, since a hidden chat can be shown again. A group deleted while its window is closed, such as from the bus's page, leaves its draft in the saved state, where it is never shown.
