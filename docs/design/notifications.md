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
- Every toast is kept as a report, an entry of the toast's severity that is saved through `SaveNotification` and dismissed through `DismissNotification`, as before. A report repeated while an earlier copy is still listed is not added again.
- A failed turn is an `error` entry about its chat, saved as a `turn-failure` and restored at start.

## Counts and Attention

Everything that counted cards now asks the store. The badge counts the entries that need the person. A chat window's attention mark and the chat list's Needs You group count the chat's questions, so a failed turn's report does not put its chat under Needs You. The Notifications window's attention mark is the badge's count, and F9 steps through the entries that need the person and that the pane's filter shows.

## Accessibility

Each row and each card leads with its glyph as well as its colour, so a type never rests on colour alone. A question that its producer has not announced is announced once, politely, through a live region the module keeps; Dinah's producers all announce theirs with a toast, which is a polite status region.
