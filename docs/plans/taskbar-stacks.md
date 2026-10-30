# Stacking the Taskbar by Workspace

Paul asked on 30 October 2026 (message 948) for windows to be stacked on the taskbar, as Windows does for one application's windows, because the taskbar's overflow had reached seventeen windows and will grow as more department-head chats and projects run in Dinah. Dinah UI proposed that stacking belongs in PUDL, since the taskbar is PUDL's, with Dinah supplying each window's group key and label. Its proposal to PUDL is in `docs/proposals/pudl-taskbar-stacks.md`.

## Paul's Decisions

Paul answered on 30 October (message 959).

- Windows are stacked by workspace, which is how Paul's projects are already arranged in Dinah, so nothing has to be assigned by hand.
- Stacking is on all the time, not only once the taskbar runs out of room, so a button never turns into a stack and back while the person reaches for it.
- The windows of the workspace chosen in the Workspace pane stay unstacked, so the work in hand is one press away and the taskbar does not rearrange itself when the front window moves to another project.

## Dinah's Side

Dinah's part is small once PUDL has the stacked button. Each window's group key is its workspace's stable ID, made safe for the characters PUDL allows in a key (letters, digits, hyphen and underscore), so that renaming a workspace never splits its stack. The workspace's name is the label, and every window in a workspace carries the same label and the same folder glyph the status bar uses for a workspace, because PUDL takes them from the group's first window. PUDL does not remember which group stays unstacked, so Dinah sets it from the workspace chosen in the pane, which Dinah already keeps, after every reload (Dinah UI, message 979). The attention and running marks each window already has are what the stack sums. Dinah's side is built after PUDL releases the stacked button, and Dinah UI takes that release into Dinah.

## How It Is Built

Dinah took PUDL 0.94.0 and turned stacking on for the window layer. The workbench gives each window its group when it makes the window's markup, when a chat window moves to another workspace, and again whenever a workspace is chosen or renamed, and the Workspaces module answers which workspace a window belongs to and what its stack is called. Most windows already record their workspace in their address, either a workspace's ID (documents, the Editor, diffs, resource viewers, extension views) or its folder (chats, subagents, failed commands, applet artifacts). A Files or Terminal window belongs to the workspace it was opened in, found from the source it was opened on or from its path under `/workspaces`, and it does not move to another stack when the person goes to a folder elsewhere. New Chat belongs to the chosen workspace, since that is where its chat would start. Group chats, Settings, Judgements, Notifications and the other panes give no workspace, so they stay ordinary buttons. The IDs Dinah makes are already safe as PUDL keys; an ID with other characters becomes "ws-" and a hash of it.

## The Host's Answers to PUDL's Revision

PUDL revised the proposal on 30 October, and Dinah UI read the revision (message 977). Architecture answered for Dinah, the host, the same day. A stack's group commands, such as closing the group, act only on the windows the stack shows, and the label counts those windows. A docked window that the stack does not list is left alone, because the person put it there on purpose and acts on what he sees. The first version has no sections inside a stack, because PUDL keeps the taskbar's order inside a stack, and sections by kind would repeat their headings when chats and documents are interleaved. Each row has its own Close button, as PUDL's revision says. Dinah UI carries these answers to PUDL.

## Everything Stacks

After the first release, Paul found on 30 October (message 1131) that his taskbar looked the same, because nearly all his windows were in the workspace chosen in the pane, which stayed unstacked, and because windows restored at start were not grouped until he switched workspaces, which was a bug. He decided that every workspace stacks, the chosen one included. Stacking only once the bar is full would need a mode that PUDL has deferred, so for now every workspace with two or more windows stacks all the time.

## Stacking When the Bar Is Full, by Workspace and Kind

Paul decided on 30 October (message 1136) that buttons should not stack until the bar begins to overflow into More, and that once it does, everything stacks by kind and workspace: the documents of a workspace together, the chats of a workspace together, all group chats together, and so on. PUDL is building that mode. Until it is released Dinah keeps every workspace stacked all the time, grouped by workspace alone. When the mode is released, Dinah switches to it, and each window's group becomes its workspace and its kind, with group chats as one stack of their own and windows of no workspace grouped by kind alone.
