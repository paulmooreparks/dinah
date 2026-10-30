# Dinah UI Audit

Dinah UI keeps this list of usability and PUDL findings on Dinah's existing screens, for Paul to prioritise. Dinah UI does not fix them; a finding Paul chooses goes to Dinah Front End. The brief is in `dinah-ui.md`.

Each finding gives the screen, the rule or problem, and a severity. A **PUDL** finding breaks a rule in PUDL's contract or in the Architectural Principles, and would block a merge if a new change introduced it. A **major** finding is a usability problem a person will meet in ordinary use. A **minor** finding is polish.

Paul decided on 30 October 2026 (message 953) to take Architecture's schedule. Front End fixes every finding marked PUDL below, together with J6, as one batch after the workspace and switching round and before the Claude Code and Codex judges. The other findings wait for Paul to choose, and Architecture recommends G1 and J5 first.

The screens were seen in the headless browser suites on 30 October 2026, in the light theme, at 1440 by 1000 pixels. The dark theme has not been looked at yet.

## Judgements Page

For this pass the Judgements suites saved no screenshots, so Dinah UI ran temporary copies of them with screenshots added and then deleted the copies.

**J1. The bars of the graph can be pressed but are drawn flat.**
Severity: PUDL.
Pressing a day's bar filters the list to that day, and the bar has `aria-pressed`, but it has no raised surface, so nothing tells the reader it is a control. PUDL requires everything pressable to be raised and nothing else to be.

**J2. The graph tells the judge's answers from the rule's by colour alone, and has no legend.**
Severity: PUDL.
Each bar is two stacked colours, the accent for calls the judge answered and the warning colour for calls the rule answered. Nothing on screen says which is which, and nothing besides colour separates them, so a reader who cannot tell the two colours apart cannot read the graph. The counts are only in each bar's accessible name.

**J3. The page's filters, its range, the chosen day and the decision in view are not in the address.**
Severity: PUDL.
They are kept in the applet's memory, so a reload or a restored window loses them and a decision cannot be linked to. PUDL's contract says that filtering, paging and the record in view are addresses.

**J4. The Corrected column marks a correction with the character "✓".**
Severity: PUDL.
PUDL draws glyphs in chrome as SVG masks and never as characters that a platform may draw differently. Empty cells in the tables also use the character "—".

**J5. The tables are wider than the window opens, and the empty state is cut off.**
Severity: major.
At the window's default size the By Kind, By Model and Decisions tables lose their right-hand columns, and each cell that does show wraps onto two or three lines ("6 / days / ago", "deepseek- / v4.1- / flash:cloud"). Before anything is judged, each empty table says "Nothing to break down" centred across the clipped width, so it reads "Nothing to break do". The tables want the window wider, fewer columns, or PUDL's stacked table (`.data-table.stack`).

**J6. A decision's detail opens below the whole list instead of beside it.**
Severity: major.
Choosing a decision draws its detail under the last row, so with fifty rows loaded the detail is far out of sight of the row that was chosen. A list beside a record is PUDL's master-detail layout, which would also give each decision its own address (J3). Architecture agreed on 30 October that J3 and J6 are fixed together as one master-detail change.

**J7. The four filters are stacked one per line above the graph.**
Severity: minor.
Kind, Answer, Reason and Model each take a full row, and they are the browser's own selects, while the rest of Dinah draws selects with PUDL's caret. A toolbar of PUDL selects in one row would leave the graph in view when the window opens.

**J8. The budget's bar is Dinah's own drawing rather than PUDL's meter.**
Severity: minor.
PUDL has had a meter with marks since 0.74.0, which shows a value against a limit with an optional mark, as the allowance does here. Using it gets PUDL's contrast, forced-colours drawing and accessible value for free.

## Judgements Tray Widget

**W1. The popup's text touches its left edge and says the day's counts twice.**
Severity: minor.
The popup's first paragraph has no padding, and it reads "Judgements: on, 12 calls today, 3 answered by the rule. Today: 12 calls, 3 by the rule." The first sentence is the item's accessible name repeated as text.

## Judgements in Settings

**S1. The on and off controls are buttons that say "On", not PUDL switches.**
Severity: PUDL.
Judgements On and each kind's On are buttons with the switch role, drawn as plain buttons whose words are the state. The Chat tab of the same window uses PUDL's switch, with its track and thumb, for the same kind of setting. A button labelled "On" also reads as an action, so a reader cannot tell whether pressing it turns the kind on or off.

**S2. "What the judge sees" is the browser's own disclosure, and its lines contain em dashes and Go types.**
Severity: PUDL.
The disclosure uses the browser's triangle rather than PUDL's `.disclosure`, so it differs from every other disclosure in Dinah. Its lines read like "members ([]string) — the members of the group", which shows a programming type to the person, and the model picker's first choice also contains an em dash. The calibration report's list of cases has the same disclosure and the same dashes.

**S3. Kinds are shown by their internal names.**
Severity: minor.
The Settings section and the page both show "chat-title" and "mention-choice". A plain name, such as "Chat Titles" and "Who a Message Wakes", would read better beside the description that already follows.

**S4. One Save button sits beside the second of two fields that it saves.**
Severity: minor.
Keep Records and Warn After are saved together by a Save beside Warn After alone, so it looks as though it saves only that field. PUDL's settings panel puts a card's buttons in a `.settings-actions` row under its fields.

**S5. The Settings window's tabs run past its edge with nothing to show that more exist.**
Severity: minor.
At the window's default width the tab bar shows Appearance, Agents, Judgements and Chat, and Terminal and Packages are cut off with no sign that they are there.

## Group Chat Composer and Notices

**G1. A member's state is a 10-pixel dot whose meaning shows only on hover.**
Severity: major.
Each member chip begins with a small glyph: a green dot for open, a smaller grey dot for closed, a ring for working and a crossed eye for open in the background. The glyphs are different shapes, so the colour is not alone, but at this size open and closed differ mainly by colour, and the words are only in the hover text and the accessible name. This is the same kind of problem as the provider switch that showed only in a hover. PUDL's status badge, which leads with the glyph and says the word, fits here.

**G2. Each message's buttons are drawn at half opacity until the pointer is over the message.**
Severity: PUDL.
The Quote, Edit and Copy buttons on every message use `opacity: .55`, so an enabled button looks the same as a disabled one, and the disabled Edit on an older message cannot be told apart from the others. PUDL's accessibility rules forbid opacity as a way to mute, and PUDL already offers row actions that show only where the reader is (`data-row-actions`).

**G3. A refused attachment is reported in a toast.**
Severity: PUDL.
"huge.bin is larger than 25 MB, so it was not attached" appears as a warning toast, which leaves by itself. The Principles reserve toasts for passing confirmations and say errors are always notices, and a person who looked away would not know the file was missing from the message.

**G4. A mention's notification card has two primary buttons.**
Severity: PUDL.
The card in Notifications for "Docs Pass mentioned you in Release Group" draws both Open Group and Dismiss as primary buttons. PUDL allows one primary button in a context, and Dismiss is the secondary action.

**G5. The count of a mention is drawn as a warning.**
Severity: minor.
The card itself leads with the information glyph, but the Notifications tab and the status bar count it with the warning triangle in amber. A mention asks for an answer and nothing has gone wrong, so the count reads as more alarming than the card.

**C3. Jump to the Latest can sit beside a row's own buttons and look like one of them.**
Severity: minor.
When the reader is scrolled back, Jump to the Latest is a small glyph-only button held at the bottom centre of the transcript. In the chat-scroll-place screenshots it lands right next to a tool row's Details button, so it reads as part of that row. Giving it its words ("Jump to the Latest"), or a place that never lines up with a row's controls, would make it read as the transcript's own control.

**G6. A decision Paul writes without a mention reaches the chats as its first line only, and can be missed.**
Severity: major.
On 30 October Paul decided in the group, mentioning nobody, that taskbar stacks group by workspace and kind (message 1136). Each member chat received only the message's first line, which did not contain the decision, and Dinah Architecture, which records decisions, missed it until Dinah UI quoted it back (messages 1162 to 1164). The note under the box does say that nobody is asked to answer, but it does not say that the other chats see only the first line, which is what makes a decision easy to lose. Two remedies are worth weighing. The note could say so plainly ("The chats see only its first line"), or a message from Paul could always reach every member chat whole, since his words are the ones that carry authority.

The note under the box when a message mentions nobody, and the line that marks where Paul last read, follow PUDL and read clearly. A small tab with a "‹" that showed over a group window's messages in one screenshot is PUDL's own collapse button for a docked window, so it is not a finding.

## Chat and Group Chat Transcripts

**C1. New activity moves a reader who has scrolled back.**
Severity: major.
Paul reported on 30 October (message 955) that new activity in either kind of chat scrolls him partway down while he is reading back. Neither chat holds the reader's place itself. Both rely on the browser's scroll anchoring, which is a heuristic, and the group chat draws its whole list again and puts back the same pixel offset rather than the same message. Architecture scheduled the fix ahead of the PUDL batch (message 957). Fixed on 30 October (message 1055). The page now records the first message in view and its offset before each change and restores them afterwards, in both chats, and the chat-scroll-place suite checks a delivered message, a streamed reply, trimmed rows and a redrawn group. Reopened the same day (message 1069). In a full run of the suites, a streamed reply moved the view by 34 pixels, so rows that change height after the page restores the place still move the reader when the page is busy. Front End fixes this before the PUDL batch.

**C2. Drop the Queued Messages was missing from the chat's Actions menu while messages were queued.**
Severity: major.
On 30 October Paul brought Dinah Front End Cleanup's window to the front, opened its Actions menu, and found no Drop the Queued Messages, although eight messages were waiting and the button showed on the composer's toolbar (messages 1076 and 1081). The command is registered to show whenever the queue shows (`drop-queue` in chat-applet.js), so the menu was most likely built before the queue appeared and was not built again. The menu must follow the queue as the toolbar does, and a suite should queue a message while the window is in front, open Actions, and find the command.

## Not Yet Seen

The dark theme, narrow windows and keyboard focus have not been looked at for any of these screens. Front End added screenshots on 30 October to the five Judgements suites and to group-notice-note, group-last-read and mention-notification, so later passes can see those screens directly.
