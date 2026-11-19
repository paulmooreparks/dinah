# Menus and the launcher

Paul asked on 19 November 2026 for every menu to be listed and compared with the menu standards of IBM, Xerox, Apple, Microsoft, GNOME and KDE, and decided the same day what the menus should become. This document has the rules he decided, the menus they lead to, and how far the work has come. The inventory, the comparison with its sources, and the mockup he used are kept with the product that commissioned them and are not in this repository.

## The rules

1. The host's menus are Chat, Workspace, Documents, View, Settings, Window and Help, in that order. They are for what the product itself keeps: chats, workspaces, and the finding and opening of documents.
2. File is an application's menu and the host has none (Paul, 19 November 2026: an application's window is the only place there is ever a file). Edit follows it, since the document applications draw both and two menus cannot have one title in the bar; this second part is taken from the first and waits for Paul's word.
3. A host menu has the same rows whatever window is in front, and the window in front adds nothing to it (Paul, 19 November 2026: the window in front should not change the host's menus, as far as that is possible and practical). A row that needs something in front, as Chat's Rename… needs a chat, is greyed with the reason when it is not there.
4. The window in front adds its own menus after Help, so the host's titles never move. Its first menu has the application's name and holds what belongs to the application as a whole. A document application then has File and Edit and the menus of its own objects. What the host carries out for the document in front (Rename…, Discard Draft…, Publish…) is in the application's File. No menu is called Actions, and none repeats the window commands.
5. View shows and hides. It starts nothing and changes nothing that is kept.
6. A command has one row in the bar. A toolbar, a pane, a pop-up menu and the launcher may repeat it, and none of them is the only place it is found. Open… is in Documents and in an application's File, since it must be reachable with no document open and belongs in every File menu.
7. A row that cannot be used is greyed and not removed.
8. A set of which one is on has radio marks, and a setting that is on or off has a check mark.
9. Labels are in title case, and an ellipsis means the command asks for something more before it acts.
10. The launcher lists what opens in a window or a pane of its own, one entry each, in alphabetical order. It lists no commands. The panes are in it, under a category of their own, so that a pane can be found and pinned from it.
11. Ctrl+P is Print…, and Find and Open… takes Ctrl+Shift+O (decided 19 November 2026; both wait until the document applications give Print… the key, since a freed Ctrl+P would otherwise do nothing).
12. The settings menu is called Settings.
13. The way out of the program is called Exit on every system. With no File in the host it is in the launcher, which opens from the product's name at the start of the bar, and Alt+F4 does it too.

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
- **The launcher** lost nine commands: New Chat, New Workspace…, New Empty Workspace, Import Workspace…, Open Document…, Find a Tool…, Keyboard Shortcuts, About and Refresh Shells. Install Packages… stays in it until a menu has a place for it. Its categories are Documents (the four document applications and Files), Chats (Decisions, Org Chart, Clean Up Chats), Panes (the six panes, Group Chats now among them), Terminals, Tools (what a product adds, and not listed when a product adds nothing) and Settings (Extensions, Theme Editor, Install Packages…). A tool that gives no category of its own gets one from a table in `title-launcher.js`.
- **Exit** is the name of the launcher's button and of the command, which were Quit.
- **Commands…** files the chat commands under Chat, the workspace commands under Workspaces, and the tools under Tools. They were all filed under View.
- **The two kinds of Terminal** list their commands in one order.

The command registry gained two things for this (`commands.js`). An entry may say `after`, the ID of the entry it stands straight after, so a command registered in another file takes its place in a group. An entry may say `group`, and a menu's list in the page may have a place for each group (`data-commands="panes"`), as View has for its panes.

### Still to come

- **Here, with nothing else needed.** The Chat menu, which takes over the chat window's Actions and has New Chat with Ctrl+N. Workspaces renamed Workspace, with New Workspace… first. Close in Window. This product's own tools giving their menu the tool's name in place of Actions.
- **PUDL.** The host's titles keep the order the host wrote them in; today PUDL puts a host title it does not know before View, which is where Settings would land. The menu with the window's name loses the window commands. The window rows are in title case, and the list of windows has radio marks. One defect was reported on 19 November 2026: a menu's rows move to the right when one of its cascades opens, if the cascade has marked rows and the menu has none (Workspaces, Recent Workspaces shows it).
- **The document applications.** They keep File and Edit. They need room in File for more of the host's rows than the eight the contract allows, Ctrl+P on Print…, and their ways of showing a document in the menu with the application's name.
- **Here, after those.** The Settings menu with Settings… and the colours, the Document menu's rows moved into the applications' menus, Find and Open… on Ctrl+Shift+O, and the launcher's Default Shell entry called Terminal, which is what it starts.

## Questions still open

1. Whether Edit stays with the applications, as rule 2 assumes, or the host has an Edit for the clipboard.
2. Whether F1 stays the list of keys or opens a help page.

## Tests

The browser suite `e2e/host-menus-order.js` covers step 1: what View, Documents, Window, Help and Workspaces hold and in what order, the themes' marks, the Chats pane's Broadcast rows, what the launcher no longer lists, its categories, and the name Exit.
