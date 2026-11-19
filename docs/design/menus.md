# Menus and the launcher

Paul asked on 19 November 2026 for every menu to be listed and compared with the menu standards of IBM, Xerox, Apple, Microsoft, GNOME and KDE, and decided the same day what the menus should become. This document has the rules he decided, the menus they lead to, and how far the work has come. The inventory, the comparison with its sources, and the mockup he used are kept with the product that commissioned them and are not in this repository.

## The rules

1. The host's menus are Chat, Workspace, Documents, View, Settings, Window and Help, in that order. They are for what the product itself keeps: chats, workspaces, and the finding and opening of documents.
2. File is an application's menu and the host has none (Paul, 19 November 2026: an application's window is the only place there is ever a file). Edit follows it, since the document applications draw both and two menus cannot have one title in the bar (confirmed by Paul the same day).
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
13. The way out of the program is called Exit on every system. With no File in the host it is in the launcher, which opens from the product's name at the start of the bar, and Alt+F4 does it too. Paul confirmed on 19 November 2026 that the launcher is enough.
14. F1 opens the list of keys, Keyboard Shortcuts, and not a help page (Paul, 19 November 2026).

## Where the work is

The target needs changes in PUDL and in the document applications as well as here, so it is built in steps.

### Step 1, built on 19 November 2026

These changes needed nothing outside this repository. After them the titles were still Workspaces, Documents, View, Window and Help, and Broadcast was in the Chats pane's own menu until step 2 gave it to Chat.

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

### Step 2, built on 19 November 2026

Paul approved this step from the mockup the same day. It needed nothing outside this repository either, and the bar now reads Chat, Workspace, Documents, View, Window, Help.

- **Chat** is a new menu and the first in the bar. It begins with New Chat (Ctrl+N) and New Group Chat…. Then come the commands of the chat in front: Rename… (F2), Continue in Another Workspace… and Pin to Workspace; Provider, Mode, Model and Effort, each a set with one mark; Start a Group Chat…, Add to a Group Chat and Team; Prompt History…, My Templates…, Export…, Show the Chat Log, Show Failed Command Summaries and Subagents; Ask Me Again About a Full Context, Load Full History and Drop the Queued Messages; the two Broadcast rows where Decisions is available; Stop and Restart the Agent; and Delete… last. A row that needs a chat is greyed with "No chat is in front." while a pane or a tool is in front, and a row the chat in front cannot use now says why, as Stop says "The chat is not working."
- **A chat's window adds no menu.** Its Actions menu is gone, since Chat has its rows. Two rows of Actions did not move: Clean Up Chats, which is a tool in the launcher, and Choose the Model…, which Model, All Models… already does.
- **Workspace** is the new name of Workspaces. It begins with New Workspace… and New Empty Workspace, then Switch To (which was Recent Workspaces), then Rename…, Refresh and Add a Source…, then Import… and Export…, then the three rows for the pane's sections, and Remove… last. Add a Source… and the section rows were in the pane only. New Chat left it for Chat.
- **Window** has the commands for the window in front as one block that ends with Close, then Next Window and Previous Window, the two document rows and the two Go to rows. Each greyed row says why.
- **View** has Refresh (F5), which reads again what the window in front lists: the chats, the workspace, the skills, the documents in progress, the requests in Decisions, or a folder in Files. It is greyed with "Nothing in front is a list that can be read again." over anything else. It replaces Refresh Chats, Refresh Workspace, Refresh Skills, Refresh Requests and Refresh, which were rows of five windows' own menus.
- **A window adds one menu, with its kind's name, or none.** A Terminal adds Terminal, Files adds Files, and the same goes for Org Chart, Theme Editor, Group Chats, a group chat's window (Group Chat) and Settings. The menu has the window's own commands and nothing else. No menu is called Actions, and none repeats the window commands, which Window has. The Workspace, Chats, Notifications, In Progress and Skills panes, Decisions and Clean Up Chats add nothing, since the host's menus have every command they had. The document applications draw their own menus and are unchanged.

The command registry gained three things for this (`commands.js`). An entry may say `menuLabel`, its words in its own menu where the title already says what it is about, so Chat shows "Rename…" and Commands… lists "Rename Chat…". An entry with `items` may say `radio`, for a flyout of which one row is on. A flyout that cannot be used is drawn as a plain greyed row with its reason, because the markup a menu is drawn from cannot grey a flyout.

The Chat rows find the chat in front through the chat's own command table (`chat-applet.js`, `menuCommands`), read once for each drawing of the menus. A chat tells PUDL when its commands change, and the Chat menu is drawn again on PUDL's `pudl:commands-change`; a menu that is open at that moment is brought up to date in place through PUDL's `refresh`, so Stop can be pressed as soon as a turn starts.

A window says how its list is read again with `reload` on what its kind's `init` returns, and `dinahWorkbench.reloadOf` gives that to View, Refresh. A kind names its menu with `menuTitle`, or with its `title`.

Two keys changed hands. Ctrl+N is New Chat everywhere, except where the application in front has a Ctrl+N of its own, which PUDL gives to the application. F2 is the host's Rename… for the chat in front; it was the chat window's own key and does the same thing.

### Still to come

- **Here, with nothing else needed.** Nothing is left that this repository can do alone.
- **PUDL.** The host's titles keep the order the host wrote them in; today PUDL puts a host title it does not know before View, which is where Settings would land. The menu with the window's name loses the window commands. The window rows are in title case, and the list of windows has radio marks. One defect was reported on 19 November 2026: a menu's rows move to the right when one of its cascades opens, if the cascade has marked rows and the menu has none (Workspaces, Recent Workspaces shows it).
- **The document applications.** They keep File and Edit. They need room in File for more of the host's rows than the eight the contract allows, Ctrl+P on Print…, and their ways of showing a document in the menu with the application's name.
- **Here, after those.** The Settings menu with Settings… and the colours, the Document menu's rows moved into the applications' menus, Find and Open… on Ctrl+Shift+O, and the launcher's Default Shell entry called Terminal, which is what it starts.

## Questions still open

None. Paul settled the last one, what F1 opens, on 19 November 2026 (rule 14).

## Tests

The browser suite `e2e/host-menus-order.js` covers both steps: the titles of the bar; what Chat, Workspace, View, Documents, Window and Help hold and in what order; the Chat rows greyed with their reason when no chat is in front; Ctrl+N and F5; the themes' marks; which windows add a menu and what it is called; what the launcher no longer lists, its categories, and the name Exit. `e2e/chat-menu-stale.js` covers the Chat menu following a chat that starts working, with the menu closed and with it open.
