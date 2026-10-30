# One Look for a Chat's State

Paul found on 30 October 2026 that a green filled circle means a chat is not busy in the group window's member list and that it is busy on the taskbar, and he asked for one colour and one glyph for each state of a chat or group everywhere in Dinah. PUDL Architect raised it with Dinah the same evening (message 1428): PUDL's own taskbar marks a running window with a green filled circle (`data-win-running`), which is one source of the clash. This document sets the one mapping that Dinah and PUDL both follow. Nothing here is built yet.

## The States

A chat's execution state is one of four, and each is always shown as a glyph, a colour and a word together, so colour is never the only signal.

| State | Meaning | Glyph | Colour | Word |
|---|---|---|---|---|
| Working | A turn is running. | An open arc that turns (still when the person asks for reduced motion), so that it differs from Idle's whole circle even when it does not move | Accent | Working |
| Waiting | The chat cannot go on until someone acts: a permission request, a question to the person, a full-context hold, or waiting for its lead. | Flag | Warning | Waiting, with the reason beside it ("Waiting for you", "Waiting for Lead", "Context full") |
| Stuck | The last turn ended in an error, or the chat's provider cannot run it. | Warning triangle | Danger | Stuck, with the reason beside it |
| Idle | The chat is open and nothing is running or waiting. | Hollow circle | Muted text | Idle |

The colours are PUDL's tokens: accent for Working, with a foreground that keeps its contrast, warn for Waiting, danger for Stuck and text-muted for Idle. PUDL has no separate information colour, and PUDL Architect proposed this assignment (message 1438). Green is not used for any of these. The words are visible wherever the space allows, on ordinary taskbar buttons and rows and in window status. A compact rail shows the glyph alone, with the words as its accessible name and its tooltip. A chat whose session is not running (closed, or retired from a team) has no execution state; it shows its lifecycle in words ("Closed", "Retired") in muted text and no glyph from this table.

Three other things look similar but are not a chat's state, and they keep their own forms so that none of them is read as one of the four above. Unread messages are a count in a badge. A provider's connection, such as "ready" in the status bar, is shown on the provider, never on a chat. A team member's presence in a group is shown by its row being there.

## More Than One Chat

Wherever several chats are shown as one (a group, a team's strip header, a lead's mark for its members, a taskbar stack, a workspace), the shared mark is the most urgent state among them, in the order Stuck, Waiting, Working, Idle. The words give the counts, such as "1 stuck, 2 working", so that the mark never hides how many are in each state. A group whose members are all idle or closed shows Idle.

## Where It Applies

Every surface that shows a chat's or group's state takes this mapping: the chat list and its rows, the group window's member chips, the team strip, a window's title-bar status, taskbar buttons and stack rows, the Window menu, previews and pickers (Add Chats, mention completion), and Notifications where a card shows a chat's state. Dinah UI makes the inventory of every such surface and how each shows state today, and Front End changes them to this mapping through one shared function that turns a state into its glyph, colour and words. Dinah UI reviews the result in screenshots of each surface in each state, in light and dark.

## PUDL's Part

PUDL replaces its boolean `data-win-running` on a window with `data-win-state`, taking `working`, `waiting`, `stuck` or `idle`, and a label for the words (`data-win-state-label`), and its taskbar, stack rows and window status draw them with the glyphs and colours above. A stack's mark follows the aggregation rule. Dinah passes each chat window's state through those attributes and stops drawing its own running dot on the taskbar. The glyphs come from PUDL's icon set, so both sides use the same drawings.

## Where the State Comes From

The engine is the one source of a chat's state, and no surface works it out for itself. Engine proposed the shape on 30 October (message 1446), and Architecture approved it with two changes (message 1447). Every chat has `life`, which is `open` while its session runs, `closed` when it has ended, or `retired` when its team retired it, and `window`, which says whether it has a window. An open chat also has `exec`, holding the state and its reason as a full phrase. A full-context hold is Waiting with the reason "Context full", because the person has to choose and nothing failed. Stuck covers an error in the last turn, a failed provider switch, and a provider or budget that cannot run the chat. The same fields are served in a chat's information, in each group's member list, in the chat directory that Add Chats, All Chats and the find and list actions read, in mention candidates and in the team snapshot. The engine also gives each group, team and lead the counts of its members in each state, together with the state it shows as a whole, chosen by the urgency order and Idle when every member is closed, so that no surface applies the order itself, and announces a change in any chat's state or life in one numbered event.
