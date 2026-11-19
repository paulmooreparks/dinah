# Menus and the launcher

Paul asked on 19 November 2026 for every menu to be listed and compared with the menu standards of IBM, Xerox, Apple, Microsoft, GNOME and KDE, and decided the same day what the menus should become. This document has the rules he decided, the menus they lead to, and how far the work has come. The inventory, the comparison with its sources, and the mockup he used are kept with the product that commissioned them and are not in this repository.

## The rules

1. The host's menus are File, Edit, View, Chat, Workspace, Settings, Window and Help, in that order, each with the meaning the standards give it.
2. A host menu has the same rows whatever window is in front (Paul, 19 November 2026: the window in front should not change the host's menus, as far as that is possible and practical). A row that acts on what is in front is a general command, such as Save, Undo, Find…, Print… or Refresh. The window in front carries it out when it can, and the row is greyed with the reason when it cannot.
3. The window in front adds its own menus after Help, so the host's titles never move. Its first menu has the application's name and holds what only that application does. It has no File, Edit or View of its own, no menu called Actions, and no copy of the window commands.
4. View shows and hides. It starts nothing and changes nothing that is kept.
5. A command has one row in the bar. A toolbar, a pane, a pop-up menu and the launcher may repeat it, and none of them is the only place it is found.
6. A row that cannot be used is greyed and not removed.
7. A set of which one is on has radio marks, and a setting that is on or off has a check mark.
8. Labels are in title case, and an ellipsis means the command asks for something more before it acts.
9. The launcher lists what opens in a window or a pane of its own, one entry each, in alphabetical order. It lists no commands.
10. Ctrl+P is Print…, and Find and Open… takes Ctrl+E (decided 19 November 2026; both wait for the File menu, since the document applications draw Print… themselves today).

## Where the work is

The target needs changes in PUDL and in the document applications as well as here, so it is built in steps.

### Step 1, built on 19 November 2026

These changes needed nothing outside this repository. The titles are still Workspaces, Documents, View, Window and Help.

- **View** has the six panes under the heading Panes (Workspace, Chats, In Progress, Skills, Notifications, Group Chats), then Show the Agent's Thinking and Full Screen, then Color Mode, Themes and Skin. Decisions, Clean Up Chats and Org Chart left it, since each opens a tool and is in the launcher.
- **Find and Open…** is in Documents, after Open…. **Commands…** is in Help. **Go to the Question** and **Go to the Chat** are in Window, after Previous Window.
- **Broadcast to Active…** and **Broadcast to All…** are in the Chats pane's Actions menu.
- **Workspaces** has Refresh Workspace and Remove Workspace…, which were only in the pane and the status bar.
- **Documents** no longer has In Progress, which View has.
- **Themes** have radio marks, and the built-in ones stand under the heading Built In where each had "(built in)" after its name. Edit Current Theme lost its ellipsis.
- **The launcher** lost nine commands: New Chat, New Workspace…, New Empty Workspace, Import Workspace…, Open Document…, Find a Tool…, Keyboard Shortcuts, About and Refresh Shells, and with them its Help category. Install Packages… stays in it until a menu has a place for it.
- **Commands…** files the chat commands under Chat, the workspace commands under Workspaces, and the tools under Tools. They were all filed under View.
- **The two kinds of Terminal** list their commands in one order.

The command registry gained two things for this (`commands.js`). An entry may say `after`, the ID of the entry it stands straight after, so a command registered in another file takes its place in a group. An entry may say `group`, and a menu's list in the page may have a place for each group (`data-commands="panes"`), as View has for its panes.

### Still to come

- **PUDL.** The host's titles keep the order the host wrote them in; today PUDL puts a host title it does not know before View. A row may show Ctrl+X, Ctrl+C, Ctrl+V or Ctrl+A without the bar binding the key. The window rows are in title case, and the list of windows has radio marks. One defect was reported on 19 November 2026: a menu's rows move to the right when one of its cascades opens, if the cascade has marked rows and the menu has none (Workspaces, Recent Workspaces shows it).
- **The document applications.** Under a host that has File and Edit they give those two titles up, carry out the general commands the host sends, and keep what only they do in their own first menu.
- **Here.** The eight menus with their general commands, the Chat menu taking over the chat window's Actions, File taking over Documents and Document, the Settings menu with Settings… and the colours, Exit as File's last row, and the two keys of rule 10.

## Questions still open

1. Whether the settings menu is called Settings or Options.
2. Whether File's last row is Exit on Windows and Quit elsewhere.
3. Whether Ctrl+E is the key for Find and Open….
4. Whether F1 stays the list of keys or opens a help page.
5. Whether the five panes leave the launcher.

## Tests

The browser suite `e2e/host-menus-order.js` covers step 1: what View, Documents, Window, Help and Workspaces hold and in what order, the themes' marks, the Chats pane's Broadcast rows, and what the launcher no longer lists.
