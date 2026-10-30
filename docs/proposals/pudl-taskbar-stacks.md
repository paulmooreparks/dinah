# Proposal: Stacking a Group of Windows on the Taskbar

Dinah asks PUDL to let a host put windows into groups and to draw each group of two or more windows as one taskbar button, a stack, which opens a list of its windows. This is a proposal for Paul to hand off to PUDL. Dinah UI wrote it on 30 October 2026, after Paul asked for it in the Dinah Cleanup group (message 948). Dinah Architecture agreed that stacking belongs in PUDL and that Dinah supplies only each window's group (message 951).

## What Happens Now

Paul's taskbar in Dinah shows one button for each open root window, and on 30 October it reported "17 more" in its overflow. He is starting to run several department-head agents and several projects in Dinah at once, so the number of windows will grow quickly. With one button per window, most of them end up behind the More button, where they are listed by title alone and the title rarely says which project a window belongs to.

PUDL already gives each taskbar button a glyph, an attention count, a running mark and a progress value (`data-win-glyph`, `data-win-attention`, `data-win-running`, `data-win-progress`). It moves buttons that do not fit into a More menu (0.81.2) and lets the reader order the buttons, keeping that order in the address as `tb.order` (0.89.0). It has no way to say that windows belong together, and so no way to draw them as one button.

Paul described the Windows taskbar as the model. There, the windows of one application share a button, hovering over it shows them, and clicking it keeps the list open so that a window can be chosen at leisure.

## What Is Asked

### The Host Says Which Group Each Window Is In

A window gets two optional attributes:

- `data-win-group`, the group's key, in the same characters as a window key;
- `data-win-group-label`, the words the stack shows, such as a project's name.

An optional `data-win-group-glyph` gives the stack a glyph by its token suffix, as `data-win-glyph` does for a window. A child window stays under its root's entry, as it does today, whatever group it is given. The host may change a window's group at any time, and the taskbar follows, in the same way as it follows a change of title.

PUDL decides nothing about what a group means. Dinah's first use will be the workspace, but the choice is the host's, and another application could group by document, by customer or by tool.

### The Layer Turns Stacking On

The window layer gets `data-win-taskbar-stacks`, with one of two values:

- `always` stacks every group that has two or more represented windows;
- `full` shows every window as its own button while they all fit, and stacks groups only when the bar runs short of room, the largest group first, before any button is moved into More.

Without the attribute, the taskbar is exactly as it is now, so no existing page changes.

The layer also gets `data-win-group-open`, the key of one group that is never stacked. Its windows keep their own buttons, while every other group stacks. The host sets and changes it. Dinah would use it for the workspace Paul is working in, so that the windows in front of him stay one press away and only the other projects fold up.

### The Stack's Button

A stack is a raised button in the window list, in the place of its first window in the taskbar's order. It shows the group's glyph, its label and the number of windows in it. It carries the sum of its windows' attention counts as one count, the running mark when any of its windows is running, and the same mark as the front window's button when the front window is in the stack. A group with a single represented window is drawn as that window's own button, never as a stack of one.

The button's accessible name says all of it, for example "Maize, 6 windows, 2 need attention". It has `aria-haspopup` and `aria-expanded`, since it opens a list.

### The Stack's List

Pressing the stack, with a pointer, a tap, Enter, Space or Down Arrow, opens a popover above it that lists the group's windows. The list stays open until the reader chooses a window, presses Escape, presses outside it, or presses the stack again. This is the click that keeps the list open in Paul's description. Opened from the keyboard, the list takes focus on its first window.

Hovering over the stack opens the same list as a preview, after a delay of about half a second, and closes it when the pointer has left both the stack and the list. Pressing the stack or anything in the preview turns it into the list that stays open. Hover is only a shortcut. Everything the preview offers can also be reached by pressing and from the keyboard, and on a touch screen the preview never appears.

Each row of the list is the window's taskbar link, with its glyph, title, detail, attention count and running mark, so that choosing it does what pressing the window's own button does: it brings the window to the front, restoring it if it was minimised, and closes the list. Each row also has a close button, shown only where the reader is, as PUDL's row actions already are (`data-row-actions`), and right-clicking a row opens that window's window menu.

The rows follow the taskbar's order. When the host gives windows a `data-win-section` label, such as Chats or Documents, the list is divided under those labels, in the order the sections first appear. Architecture proposed sorting a workspace's stack by kind in this way.

### Order, Overflow and the Address

The stack takes the place in the taskbar of its first window in `tb.order`. Moving a stack, by dragging it, with Alt+Shift and an arrow key, or in Arrange Taskbar, moves its windows together as a block. Moving a window inside the stack's list moves it within the group. `tb.order` remains a list of window keys, so the address keeps its present form and an address written before stacks reads the same.

Whether the list is open is momentary, like an open menu, and is not part of the address. Stacking is worked out again from the windows' groups each time, so it needs no state of its own.

A stack counts as one entry when the bar runs short of room. In the More menu a stack becomes a submenu of its windows.

The window switcher (Ctrl+Tab) is unchanged. It lists windows in the order they were last in front, one by one, because a person switching back to the window they were just in should not have to open a stack first.

### Commands for a Group

The stack's context menu offers Minimise the Group, Restore the Group and Close the Group. Close the Group closes each window as its own Close would, so a window that refuses to close stays, as Close All does today. `pudlWindows` gains `group(key)`, which returns the keys of a group's windows, so that a host can offer the same commands elsewhere.

### Words

The new words are on the layer, through the existing `data-win-text-*` mechanism: the stack's name with `{label}`, `{n}` and `{attention}`, the three group commands, and the list's name.

## What PUDL Has and Lacks

PUDL already has most of the parts this needs. It has taskbar links with their glyph, attention, running and progress marks, the More button and its menu, taskbar ordering with its address parameter, popovers with their placement and focus, row actions shown only where the reader is, badges, tooltip delays, and the window menu.

It lacks five things:

1. an attribute that puts a window into a group, with the group's label and glyph;
2. the stack's button, with the counts and marks of its windows combined;
3. the stack's list, which opens on a press, opens as a preview on hover, and stays open on a press;
4. ordering and overflow that treat a stack as one entry while `tb.order` keeps listing windows;
5. the group's commands, and the `group(key)` function behind them.

## Rules This Keeps

The stack is raised because it is pressed, and the windows in its list are links because each one goes somewhere. Its counts are badges that lead with their glyph, so no colour stands alone. The list can be reached by keyboard and by touch, not only by hover. Nothing about stacking goes into the address except the window order that is already there, and an existing page that does not opt in is unchanged.

## Dinah's Use of It

Paul decided two questions on 30 October 2026 (message 959). Dinah stacks `always` rather than only when the bar is `full`, because with `full` a button would turn into a stack and back as windows opened and closed near the bar's limit, sometimes as Paul reached for it. The open group, whose windows keep their own buttons, is the workspace chosen in the Workspace pane rather than the workspace of the window in front, so the taskbar does not rearrange itself each time Paul moves between projects.

PUDL's design for the work is in `docs/proposals/taskbar-stacks.md` in the PUDL repository. On 30 October PUDL Architect accepted Dinah's answers as the host (message 982). A stack's commands act only on the windows that stack shows, and its label counts only those, so a docked window the stack does not list is left alone. Moving a stack in the taskbar still gathers all of its group's windows in the shared order, but that changes only their order and never opens, closes or minimises anything. The first release has stacking all the time and the host's unstacked group, and sections are optional.

Dinah groups windows by workspace. Architecture takes Dinah's side of the work: which key and label each window carries, and keeping `data-win-group-open` set to the chosen workspace.

## Second Request: Stacking Only When the Bar Overflows

Paul changed the rule on 30 October, after using 0.94.0 (messages 1131 and 1136). Buttons should not stack while every window fits on the taskbar. Once the bar would begin to overflow into More, every window should stack, by workspace and by kind together: the documents of one workspace, the chats of one workspace, all group chats, and so on. This is the `full` mode that PUDL left out of 0.94.0 until it had rules for measuring and stability, and Dinah now asks for it, with a different stacking rule from the one this proposal first described.

### What Is Asked

- **One switch for the whole bar.** In the new mode, the bar shows every window as its own button while they all fit, and stacks every group once they would not. It does not stack the largest group first and the others later, so there is one point at which the bar changes, and the person learns it once.
- **When it switches.** The bar stacks when the separate buttons would need the More button, and returns to separate buttons only when all of them would fit with room to spare, for example the width of one more button. The gap between the two points stops the bar from flipping between the two forms as a window opens and closes near the limit, which was Dinah's reason for choosing `always` in the morning.
- **No open group in this mode.** Paul asked that everything stack once the bar overflows, so `data-win-group-open` does not apply while the new mode is stacked. PUDL may keep honouring it in `always`.
- **Overflow after stacking.** If the stacks themselves still do not fit, they go into More as stacks already do.
- **What does not change.** Everything else in 0.94.0 stays as it is: the stack's button and list, commands acting on the windows shown, and the order kept as window keys in the address.

### Dinah's Side

PUDL Architect recorded a concrete design for this mode the same day (message 1139). The bar switches all groups together at the point its ordinary buttons would need More, after the existing shrinking and tray compaction. It returns to separate buttons only when there is room to spare equal to the widest button's minimum width plus one gap. Dinah UI suggested holding a switch while a stack's list, a menu, Arrange or a drag is in use, and applying it afterwards.

A window's group becomes its workspace and its kind together, such as the chats of the Dinah workspace, with a label that says both ("Dinah Chats") and the kind's glyph. Windows that belong to no workspace form groups by kind alone, such as "Group Chats". Architecture builds this once PUDL ships the mode. Until then, Dinah stacks every workspace with two or more windows all the time.
