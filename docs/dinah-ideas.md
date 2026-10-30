# Dinah Tasks

## Bugs/Annoyances

- Confirm before exit if any chats are active or any other actions are in progress which cannot be paused and restarted on launch
- Skills support in Chat. Right now, it doesn't find any skills.
- App-wide font scaling. Let the user define a font size (of some kind) for Dinah Desktop and all applets and controls are displayed relative to that. See if this is feasible and if there is prior art to follow.
- Specify/design/implement/review/test flow
- A question posted in a group chat without a mention reaches nobody, so a chat that asks one and waits stalls for good (Dinah Cleanup, 29 October 2026, both working chats waited an hour). Something should catch this: a group message that reads as a question with no mention could be refused or warned about, or a chat could be told when its message woke nobody.
- Switching a chat's provider can lose group membership and misroute mentions (Dinah Cleanup, 29 October 2026). After Architecture switched from Claude to Codex, sending to the group failed because the new session was not a member; the bus listed two Architecture identities. Joining restored group access, but Paul's message 281 mentioned Dinah Architecture while its delivery to the Codex session said it did not mention that session, and no automatic answer followed. Engine Cleanup also reported needing to join after its provider switch (message 283). The studio self-link still had no handoff limit, so the earlier hop-limit fix does not resolve this issue. A provider switch should preserve the logical chat's group membership and route mentions and wakeups to its current session without manual rejoining. Duplicate or stale bus identities are a suspected cause; the routing failure's root cause remains unverified.
- Front End Cleanup reported that the bus rejected its direct Architecture mention because two group memberships had the same title and workspace (message 285, 29 October 2026). Mentioning everyone reached the current Architecture session. This confirms duplicate memberships affect addressing; it does not yet establish the full cause of the missed wakeup above.
- Engine Cleanup reported losing its accepted group membership again between messages and needing to rejoin (message 295, 29 October 2026). Rejoining is therefore not yet a reliable workaround for the provider-switch issue.
- Adding a checklist still causes the Visual editor to drop into text-editing for the section on reload or after switching to Text and back
- Cut and deletion of code-backed items in the editor. For example, I can't just click on and delete and image, because it drops me into the code editor
  - Specifically, clicking an image should bring up an image dialog to allow setting/changing the image and its properties
  - More generally, clicking a rendered but otherwise unsupported (in the editor) item should bring up an in-place code editor, as it does today, but I need a way to select the entire item and cut/delete it.
- Setting to choose the initial view for an empty document. Right now, it defaults to Split, which is never what I want. I want to default to Visual.
  - Honestly, we should be able to set the default by MIME type or extension or some nexus thereof.
- The title of a document doesn't change after I open a new document and then save it.
- The terminal title bar should reflect the running application
- The taskbar doesn't show the terminal title bar correctly.
- Double-clicking a file in Files should default to opening the editor, if possible, or if it can't discern a MIME type it should ask me what applet should handle the action.
- The Files applet needs right-click, on items in the tree and on items in the directory listing.
- Add Alt-Up-Arrow and Alt-Down-Arrow support for moving lines or groups of selected lines up and down
- Rectangular selection in editor Text mode

## Extensions


## Done

- GIT INTEGRATION!!!!1!!!1!
- Full-screen support
- **DONE** New color scheme specific to Dinah
- **DONE** Make the frying pan the app icon
- **DONE** Adjust the image in the about box. It should have the same padding from the top that it has from the left, to make it look balanced.
- **DONE** There is too much space above the first line in the editor. First, there's too much top padding for H1, but there's also just too much padding at the top.
- **DONE** The YAVCHN logo does not show up in the YAVCHN repo's README.md when I load it into the Visual editor. My guess is that it can't render a path.
- **DONE** Fix local-link Markdown rendering in chat responses:
  - Links don't always render
- **DONE** Add a "working" chip in the title bar for chats that are active.
- **DONE** Checkbox editing in Visual mode is verified by mouse and keyboard, including Markdown synchronization and Undo.
- **DONE** Deliver a message from one chat to another as steering when the receiving chat is busy and its current provider/model can accept steering. If steering is unavailable, queue it for delivery after the active turn; an idle chat receives it immediately.
- **DONE** In chat, when I paste or attach an image, I want to be able to click on it after it scrolls up and see the image I've pasted. Right now I only see a bit of text in the chat with the name of the image I pasted.
