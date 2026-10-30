# Proposal: The Menu Bar Follows Changes to the Window in Front

Dinah asks PUDL to keep the menu bar's copy of the front window's menus current. The menu bar should follow a new title given by `pudlWindows.retitle()`, and the window's commands as they stand when a menu opens. Dinah UI wrote this on 30 October 2026, after Dinah found two cases of the same gap. PUDL Architect recorded the first case the same day as a separate follow-up after taskbar stacks, and asked Dinah to keep its workaround until then (message 1015). It took the second case into the same follow-up (message 1085). It prefers to read the front applet's current commands each time a menu opens, and its design will also cover ribbons, keyboard shortcuts and commands that change while a menu is already open. It asked for Dinah's queued-messages suite, which Dinah UI sends once Front End has written it.

## The First Case: A New Title

The README says that `pudlWindows.retitle(key, title, detail)` changes a window's title, and that the title bar's spoken name, the dock tab and the list row follow. The menu bar's front menu, whose title is the front window's title, does not follow. It keeps the old title until something else makes the menu bar rebuild, such as another window coming to the front.

In Dinah, renaming a group chat from its window's Actions menu calls `retitle`. Straight afterwards the title bar and the taskbar button show the new name, while the menu bar still shows the old one. Dinah's group-rename browser suite reproduces it: rename the window, then read the front title in the menu bar.

A page with no Dinah in it shows the same thing. Open a window with a menu bar on the page, call `pudlWindows.retitle(key, 'New Title')` while that window is in front, and read the menu bar's front title.

## The Second Case: A Command That Appears Later

A Dinah chat offers Drop the Queued Messages only while messages wait to be sent, so its applet's commands include that command only then. On 30 October Paul brought a chat with eight queued messages to the front and opened its Actions menu from the menu bar, and the command was not there, although the same command showed on the chat's toolbar. The menu bar appears to have kept the commands it read when the menus were assembled, before the queue appeared. Dinah's chat-menu-stale browser suite reproduces it. It reads the front chat's Actions menu from the menu bar, then changes the chat's commands by starting a turn and by queuing a message, and reads the menu again.

The contract describes the commands an applet returns, and `pudl:window-menu` lets content add commands as the window menu opens. It does not say when the menu bar reads an applet's commands again, and it offers an applet no way to say that its commands have changed.

## What Is Asked

1. When `retitle` changes the title of the window in front, the menu bar should take the new title, just as the dock tab does. The README's sentence on `retitle` should then list the menu bar among the places that follow.
2. When a menu in the menu bar opens, it should show the front applet's commands as they are at that moment, as the window menu does. If PUDL prefers not to read them on every opening, a documented call such as `opts.commandsChanged()` would let an applet say that its commands have changed. The contract should then say which of the two PUDL does.

## How Dinah Works Round It

For the first case, Dinah calls `pudlMenubar.refresh()` straight after each `retitle`. The contract describes that function as rebuilding the menu bar after a change the script did not see, and it does put the new title in the bar. Once PUDL follows `retitle` by itself, Dinah removes the call, and the group-rename suite will show whether it came out cleanly.

The second case has no workaround. Dinah Front End found on 30 October (message 1109) that `pudlMenubar.refresh()` rebuilds the bar, but an Actions panel built earlier still shows the commands it had then, so a command that appears later never reaches it. That falls short of what the contract says `refresh()` does, so the second case is a defect as well as a request. The chat-menu-stale suite is held back until PUDL fixes it, and Dinah adds no stand-in of its own meanwhile. PUDL Architect agreed the same day (message 1114): its menu bar promises fresh rows each time a menu opens, so it treats this as an urgent defect in the follow-up. It asked Dinah to keep chat-menu-stale as a failing regression and not to work round it through PUDL's internal panels.
