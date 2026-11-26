# Notifications

Paul asked on 26 October 2026 for the Notifications item in the taskbar to open a list of the notifications themselves, each shown by its glyph, its colour and a description of at most two lines, instead of a sentence that only says notifications are waiting. He asked for the change to be coherent, without duplicated logic, and portable to other programs. This note is the design of record for the notifications model, the taskbar's list and the Notifications pane.

## Why the Old Shape Had to Go

Until this change the list of notifications existed only as cards in the pane's DOM. Every producer built its own card and put it into the pane: a chat's permission question, an applet's request and its result, a report kept from a toast, and a failed turn's report. Counting what needs the person, filtering, the taskbar badge, the chat list's Needs You group and each chat window's attention mark all worked by reading those cards back out of the DOM. A second view of the same notifications, such as the list Paul asked for, would have had to read the cards again or build a second copy of each one, so the list lives in a model now and the views draw from it.

## The Model

`frontend/notices.js` holds the list of notifications and nothing that belongs to Dinah. `Notices.create(adapter)` returns a store. Each entry has these fields:

| Field | Meaning |
| --- | --- |
| `id` | A stable identity; adding an entry with an existing ID replaces it. |
| `type` | `question` (it waits on the person), `error`, `warning`, `success` or `info`. |
| `glyph` | A PUDL glyph name, by default the type's own. |
| `description` | The short text the compact list shows. |
| `title`, `body`, `details` | The full card's heading, its text, and an optional list of lines. |
| `time` | When it happened, by default when it was added. |
| `source` | A label for what it is about, such as a chat's title. |
| `target` | What it is about, opaque to the module; the adapter opens it. |
| `needsPerson` | Whether it is counted on the badge; by default true for questions and errors. |
| `actions` | Buttons for the full card, each a label, a function and whether it is the primary one. |
| `dismissible`, `dismiss` | Whether the card has Dismiss, and an optional function that dismisses this entry where it is kept. |
| `clearable` | Whether Clear All takes it; by default everything but a question. |
| `render` | For an entry whose full form is rich, the producer's function that fills the full card. |
| `extend` | An optional function called with the finished full card, with which the producer adds a control whose state it keeps itself, such as a button that can be disabled with a reason. |
| `className`, `data` | Extra classes and `data-` attributes for the full card. |
| `announced` | True when the producer has already announced the entry, as a toast does. |

The store offers `add`, `update`, `remove`, `get`, `list` with an optional test, `subscribe` for changes, `count` (the total and the number that need the person), `open`, `dismiss` and `clear`. Questions come first and everything else follows, each group newest first, and both views use that one order.

The module persists nothing. The adapter passed to `create` supplies what belongs to the application:

| Adapter member | Purpose |
| --- | --- |
| `colour(tone)` | The CSS colour of a tone (`error`, `warning`, `success` or `info`); PUDL's tokens are the default. |
| `style(card, tone)` | Optional extra styling of a full card, such as the application's own severity classes. |
| `open(entry)` | Shows the entry somewhere other than the pane, and returns true if it did. |
| `openTarget(entry)` | Opens what the entry is about; the full card offers "Open" followed by the source. |
| `showAll()` | Opens the pane. |
| `dismiss(entry)` | Records a dismissal where the application keeps its notifications. |
| `clear(entries)` | Records Clear All in one call, before the entries go. |
| `error(err)` | Reports a failure, such as a dismissal that could not be saved. |
| `formatTime(date)` | Optional formatting of an entry's time. The card shows it in a `time` element of class `notices-time` with the entry's time as its `datetime`, so the host can write it again when its format changes; Dinah formats it in the computer's regional format (`regional.js`). |

## The Views

Both views draw from the store's ordered list and keep one element per entry, rebuilding an element only when its entry changes. That keeps focus, scroll position and anything typed into an applet's form while other entries come and go.

**The compact list** is drawn by `store.mountList(host, options)`. In Dinah it fills the popover of the taskbar's Notifications item, which is a PUDL menu panel anchored to the item. Each row is a menu action button showing only the type's glyph in the type's colour and the description, clamped to two lines with CSS. The row's accessible name is the type, the whole description and the source, and its tooltip is the whole description, so the clamped text is never lost. Activating a row opens the entry. For a chat's permission question that means the chat with the question in view; for any other entry, Notifications with the entry scrolled into view and briefly marked. The popover takes focus to its first row when it opens, PUDL's menu script moves between the rows with the arrow keys, Enter activates a row, and Escape closes the popover and returns focus to the item. Below the rows are two commands, Show All, which opens the pane, and Clear All…, which asks first as the pane's own button does. An empty list says "No notifications." in one line.

**The full pane** is drawn by `store.mountPane(host, options)` into the Notifications window. Each entry is a PUDL notice with its glyph and its colour, its title, body, details, time and source, its actions, and Dismiss where it can be dismissed. An entry with its own renderer is filled by the producer instead, and the module then adds the source's Open button and Dismiss to the card's actions. The pane's filter, by type and by text, is the host's, and it is kept in the UI state as before; the view hides the cards the filter rejects and reports how many show. `store.card(entry)` builds the same full card for any other place, so another view can show the same entries without cloning the pane's cards.

**The item itself** shows the number of entries that need the person, as before.

## Producers

Every producer adds entries to the store and none builds into the pane.

- A chat's permission question is a `question` entry whose renderer draws the question's options. The chat's own copy beside its composer is drawn from the same request and answered through the same call, and it uses the question's glyph. A question goes when it is answered, when its chat is removed, and when its agent stops.
- An applet's request is a `question` entry whose renderer draws its form. Its result is an entry of the result's severity, and its Dismiss dismisses the result in Go.
- An error toast is kept as a report, an entry of the toast's severity that is saved through `SaveNotification` and dismissed through `DismissNotification`. A report repeated while an earlier copy is still listed is not added again. Any other toast passes and is not kept, unless its caller passes `keep` because it reports something the person may have missed and needs to find again, such as a window closed by a change made elsewhere or the parts of a publish that did not go. Until 18 November 2026 every toast was kept, and the person found the list filling with confirmations such as "Link copied."
- A toast that only confirms what the person can already see is not made at all: a copied link, a renamed file, a restarted agent, a saved template, a removed workspace, a changed setting, a saved correction, and a deleted chat whose dialog already said where it goes. A toast stays, passing, when it says something the screen does not show, such as where a file was saved, what an import brought, or what to do next.
- A failed turn is an `error` entry about its chat, saved as a `turn-failure` and restored at start.
- The card of a notification that belongs to an open `ask_person` request has Dismiss Request…, and a task question's card has Dismiss Question…. Each asks first with the Decisions window's dialog and then dismisses the request itself through `DismissRequest`, as `docs/design/request-settlement.md` describes. Dismiss on a card, Clear All and opening a request from its card remove the notification only and never dismiss the request (Paul, 17 November 2026). A request's card is taller than other attention cards by a fixed amount, which gives the line under its buttons one height in every state.

## Counts and Attention

Everything that counted cards now asks the store. The badge counts the entries that need the person. A chat window's attention mark and the chat list's Needs You group count the chat's questions, so a failed turn's report does not put its chat under Needs You. The Notifications window's attention mark is the badge's count, and F9 steps through the entries that need the person and that the pane's filter shows.

## The Chats Busy Count

The count depends on the documented observations in [Background Work](background-work.md#provider-coverage-and-known-limits). Claude hooks and reported shell tasks, and supported Codex server snapshots, can keep a chat busy after its turn. Monitor tasks, detached work and providers without a documented host lifecycle can remain unseen. An empty count cannot certify that such unreported work stopped. Failed observations retain previously reported work. The count has no existing tooltip or help text to extend, so this change adds no screen wording.

The Chats window's tab shows how many chats are busy, beside the Notifications tab's count of what needs the person. Paul asked to see at a glance how many agents are working, approved the mockup in `docs/mockups/chats-busy-count.html`, and settled its open points on 26 November 2026.

A chat is busy while a turn is in progress or while any background task or subagent is still running. The count covers every open chat this window runs, in every workspace, whichever workspace the list shows, because the tab belongs to the window. It ignores which group the list puts a chat in, so a chat under Needs You that still has a background task counts, and a stuck chat with a task running counts too. The count therefore need not equal the Working group's count. A chat running in another window has no execution state here and is not counted. When the count is gone, no supported observation reports running work. The provider limits above qualify the intended rule that zero means nothing is running.

The count comes from the engine's chat states (`agent:chatstate`, or `ChatStates` after a resync), which give each open chat its execution state with the number of background tasks and subagents still running (`cmd/dinah-desktop/chat_state.go`). `dinahChatState.busyCount` counts the entries whose life is open and whose execution state is Working or Background or has a task or subagent left. The Chats browser writes it on the Chats window as `data-win-count`, with `data-win-count-label="busy"`, whenever the states change and when the window opens, and removes `data-win-count` at 0. It writes an attribute only when its value differs.

PUDL draws `data-win-count` on a window's tab as a plain neutral badge, hidden at 0, with the accessible text "{n} busy" taken from the label. Dinah draws no badge of its own. Until a PUDL release that draws the attribute is taken, the attribute shows nothing, which Paul accepted.

Paul's decisions of 26 November 2026 are listed in Table 1.

**Table 1** *Decisions on the Chats Busy Count*

| Question | Decision |
| --- | --- |
| Room for the badge | None is reserved. The Chats tab widens when a count appears, as the Notifications tab does today; the Notifications tab stays as it is. |
| Badge style | A plain neutral badge, not the warning-coloured attention badge. |
| The list's hold | Kept. The brief gap between the count changing and the held rows moving is accepted. |
| What counts | Every open chat in every workspace with a turn in progress or any background task or subagent running, whichever group it is listed in. |
| Zero | No count, which means nothing at all is running. |

*Note.* The rule and the attribute are checked by the browser suite `e2e/chats-busy-count.js`.

## Accessibility

Each row and each card leads with its glyph as well as its colour, so a type never rests on colour alone. A question that its producer has not announced is announced once, politely, through a live region the module keeps; Dinah's producers all announce theirs with a toast, which is a polite status region.
