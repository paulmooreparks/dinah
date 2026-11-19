# Which chats the bus counts as open, and how one is addressed

The bus lists the chats that are open to every agent when its session starts, and an agent sends to a chat by its title. On 19 November 2026 a message to a team's lead was refused because two chats of one workspace had its title. One was the lead. The other was a session that an agent program had started in a terminal on 3 November and that had never said it ended. Paul could not find that chat in the window to rename it, because it was never the window's. This document has the rules that came of it.

## What ends a chat

- **A chat in the window** is ended by the window, when the chat is closed or the window shuts down. A window that starts also ends the chats an earlier window left open (below).
- **A chat started outside the window**, by an agent program in a terminal or an editor, is ended by the program's SessionEnd hook.
- **A chat started outside the window that has not been seen for seven days** is ended by the bus (`bus.StaleAfter`, `internal/bus/stale.go`). A program that is killed, or a computer that is switched off, never runs the hook, and before this rule such a chat stayed open for good. The bus looks for them whenever a chat starts, so the list a new chat is given has none.

Seen means that the chat registered, took a delivery, or that the program serving its tools said it was still running. That program, `dinah mcp`, says so once an hour for the chat it serves (`bus.HeartbeatEvery`), so a chat that sits idle and can still be woken is not taken as ended. A program older than this rule sends no heartbeat, and its chat is seen only when it takes a turn.

A chat that the bus ended is open again as soon as it is seen, which is how any ended chat is opened again. The heartbeat does not open one: a chat is opened by what it does, and not by a program that outlives it.

On the day of the change the bus had 14 open chats that were started outside the window, and 12 of them had not been seen for between 2 and 28 days.

## How a chat is addressed

A chat is addressed by its title. Where chats of two workspaces share a title, the workspace in parentheses tells them apart, as "Fix the Parser (studio)". Where chats of one workspace share a title, the workspace does not, and each is addressed by its ID on the bus. `send` has always taken an ID, and nothing told an agent so. Now:

- the refusal for a shared title gives each chat as it can be sent to: with its workspace where that is enough, or with its ID and when it was last seen;
- the list of chats (`sessions`, and the list a new session is given) gives the ID and when the chat was last seen for chats of one workspace that share a title, and for no other chat.

## The window's own chats after a window that did not shut down

The window ends a chat on the bus when the chat closes and when the window shuts down in the ordinary way. A window that is killed, or replaced by a new program while chats are open, ends none of them. Eight chats of one workspace had counted as open since 23 to 25 October 2026 for that reason, and one more in another workspace since 10 November.

A window that starts now ends every chat of the window that the bus still counts as open and last saw before the window started (`EndWindowSessionsSeenBefore`, called in `startup` when the bus is opened). That leaves the bus as an ordinary shutdown would have left it. Each chat is open again when it opens, and a chat the starting window has already opened is left alone, since the bus saw it after the window started. Only one window runs at a time, so no other window's chats are ended by this.

A closed chat of the window can still be sent to by its title, and its message waits for it. So two closed chats of one workspace with one title are still refused, with their IDs. I looked on 19 November 2026 for chats that share a title because they are earlier stages of one chat that changed its agent program, which would have called for a rule that only the newest stage has the title. There are none: a chat that changes its program takes its ID on the bus with it (`moveBusSession`), and has done since before that date. The one title that several of the window's chats shared was five separate chats that the person had given the same name in October. Renaming or deleting them in the window is what tells them apart, and no rule was added.
