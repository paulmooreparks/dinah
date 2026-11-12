# One thing, one window

Paul reported on 12 November 2026 that a chat had two windows, one of them empty with the chat's name and state on it, and that the pinned taskbar buttons of two chats read "Loading chat…" although the chats' windows were open and titled. This note records the cause and the rule that mends it.

## What went wrong

A window's key is made from its item: the kind, the workspace and the path (`encode` in `workbench.js`). For a chat the workspace is the chat's folder as a string, and the path is its session. Windows writes one folder with a small or a capital drive letter, depending on where the string came from, so one chat could have two keys. Everything that compares keys then treated one chat as two things.

- A pinned button saved with one spelling never matched the window opened with the other. PUDL showed the button as a destination with no window, under the kind's placeholder title.
- A chat that was continued under a new session (a change of provider) hands its old window over to the continuation (`openSession` in `session-lifecycle.js`, `follow` in `chat-applet.js`). The hand-over looks for the old window by a key made from the folder the engine reports. With another spelling it found no window and waited for one that never came, so the old window stayed open with nothing in it. Its pinned button stayed with the old session as well.

## The rule

A kind may say what its items are the same thing by, with `identity` in its registration, a function of the item. `canonical` in `workbench.js` reads an item of such a kind as the item of the window already open for the same thing, or else of the pinned button for it. `source`, `key` and `pinReference` all read items that way, so every way of asking for a thing reaches its one window.

A chat's identity is its session in its folder, with the folder compared without regard to case, to which slash it uses or to a slash at its end (`chatIdentity` in `chat-applet.js`). The session is what makes two items the same chat. The folder is compared only so that a chat continued into another workspace, which keeps its session and changes its folder, is still a window of its own. Nothing here asks the file system whether two paths are one folder.

A kind that says nothing keeps one window for each key, as before.

## Pinned buttons follow their thing

`taskbar-pins.js` gives a pinned button the key its thing has now, in four cases.

- A window is given a new key (`rekey` in `workbench.js`): its button goes with it.
- An old session's window closes in favour of the continuation's (`follow`): the button becomes the continuation's.
- At the start, and when a window opens later, a button whose thing has an open window under another key takes that window's key. This heals the pins saved before this change, with no action by the person.
- At the start each pinned chat is asked for its newest session (`movePinnedChats` in `chat-applet.js`, the engine's `ResolveSessionContinuation`), and a button saved for a session the chat was continued from is given the newest one. No window is opened for this.

A pinned thing whose window is closed has no kept title, so its kind is asked for one (`titleOf`) before the placeholder is used.

A button whose new key is pinned already is dropped, so no thing has two. PUDL keeps the pins, their order and their references; the three are rewritten together and given back through `configureTaskbar`, which PUDL documents as the way to set them.

## A window that is still being opened

At the next start after the first version of this change one chat's old window was still left half-followed, with its log emptied and the error "The chat window could not be renamed" (Paul, 12 November 2026). The hand-over asked PUDL whether the continuation's window was open. At a start PUDL opens the windows of the layout one after another, and does not list a window as open until it is; it also refuses to give a window the key of one it is opening. So the old window found no open window, tried to take the key, and was refused part of the way through.

The hand-over now also counts a window the workbench has made markup for (`known` in `workbench.js`), which it does for every window of the layout before any opens. It opens that window, which waits for one being opened and brings an open one forward, and then closes the old one (`handOverTo` in `chat-applet.js`). A rename that PUDL still refuses ends the same way and leaves nothing half-done. The window is renamed and closed by the key PUDL has for it, read from the window itself. A group chat's window that finds its group's own window does the same (`bus-applets.js`).

This start could not be reproduced in the browser suites, where a window opens at once. `e2e/chat-continued-pinned.js` and `e2e/chat-continued-opening.js` cover the hand-over to a window that is open and to one whose markup exists and is opened at that moment; the refusal itself was seen only in Paul's log.

## Checks

`e2e/one-thing-one-window.js` covers the rule with a fixture kind: two spellings open one window, a pinned button is found by the other spelling, a window given a new key keeps its button, and a button moved onto a pinned thing leaves one. `e2e/chat-continued-spelling.js` is Paul's case: the old session's window, pinned and spelled one way, is handed over to the continuation's window spelled the other way. It fails on the code before this change, with the old window left open. `e2e/chat-pins-follow.js` starts with a pinned button for a continued chat's old session and no window, and finds the button moved to the newest session with no chat opened.
