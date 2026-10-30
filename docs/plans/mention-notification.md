# Telling Paul When a Chat Mentions Him

This is the shape of the first of the three group features Paul asked for on 29 October 2026 (message 588: "I need a notification when I'm mentioned"), written by Dinah Engine Cleanup at Architecture's request, before any code. It answers three questions: where a mention of the person is detected, what the card in Notifications shows and does, and what the chats are told so that they mention the person when they need him. The DESIGN.md change is listed at the end and made in the commit with this document. No code for it exists.

## What the code does today

An agent mentions the person by putting `paul`, or the person's name, in the `mentions` of the bus's `send` or `reply` tool. The bus then adds a delivery of kind `human` for the person, so the message is in the person's inbox as unread, and keeps `paul` in the message's list of mentions (`resolveMentions` in `internal/bus/rooms.go`). A message to the group that does not mention the person gets a delivery of kind `human_room` instead, which only counts as unread in that group's window. A message sent to `paul` directly is in the inbox the same way.

The Group Chats pane shows the inbox and a "New" badge, and the group's window marks its messages read when it is shown. Nothing reaches Notifications, and nothing reaches the operating system. The chats are told that the mention field wakes chats and, in the field's description, that `paul` "puts the message in the person's inbox". They are not told to use it to reach the person, and chats have been writing the person's name in the text instead, which mentions nobody.

## Where a mention is detected

At the bus, in its data, so that it works whichever process sent the message. A mention of the person is exactly an undelivered delivery of kind `human` for a message that an agent sent. That is a fact the bus keeps, whether the sender was a window, the command line or a future daemon, and the window does not have to be the one that sent it.

The window watches the bus already: its change counter (`bus:changed`) rises on every write by any process. On each rise the window asks the bus for the person's unread messages from agents that it has not yet made a card for. A new call, `Bus.Mentions(ctx, since)`, returns them, newest last, each with its sender's chat and workspace, its group if it has one, its text and its number. It changes nothing.

A card is not made twice, and no cursor has to be kept, because the card's identity is made from the message's number (`mention-<number>`) and `SaveNotification` already returns an existing item with the same identity. To keep an old message from making a card again after the store has pruned the one it had (a week, `notificationRetention`), the call only looks back a day. The first look after the window starts therefore makes cards for what arrived while it was closed, if it is still unread and a day old at most.

The same pass does the reverse: a mention card whose message the person has since read, in the group's window or the inbox, is dismissed, so the card does not outlive the need. A card the person dismisses does not mark the message read.

## What the card shows and does

The card is a notification of kind `mention` and severity `info`. Its title is the sender's chat as the person reads it, and the group: "Dinah Architecture mentioned you in Dinah Cleanup", or "Dinah Architecture sent you a message" for a direct message. Its body is the first 400 characters of the message, as the arriving cards cut them. It carries the sender's chat, its workspace, the group's key and the message's number, in fields the notification gains (`group` and `message`), so the page can act on it.

Its action is to open the group's window, or the inbox for a direct message, scrolled to that message. The scroll to a message is the second of the three group features, "a way back to where he stopped reading", and until it exists the action opens the window, which marks its messages read and so dismisses the card by the rule above. Its button reads Open Group.

The card counts in the Needs You total and its badge like any notification. When the window is not the active one and Notify When Needed is on, the existing host notifier also shows the operating system's notification with the same title and body, and asks for the taskbar's attention, as it does for a question (`hostNotices.show`). A window that has the focus shows no host notification, as now.

Cards are made one for each message. A burst of mentions makes a burst of cards. I would rather see that happen once before adding coalescing, and Paul can say if he wants a group's mentions within a minute to become one card.

## What the chats are told

Two places carry the instructions every chat is given (`internal/communication/protocol.go`, the text for a chat in the window, and `internal/mcpserver/server.go`, the bus server's own), and one the tool's field description. Each gains the same instruction, in this sense:

> When you need {person}, mention them: put `paul` in the mentions of your send or reply, beside any chats you are asking. That puts the message in front of them as a notification. Writing their name or an @ and their name in the text mentions nobody. Mention them when you need a decision or an answer from them, or when you have something they asked to be told, such as work that is ready for them. Do not mention them to say thanks, to report progress that needs nothing from them, or to answer a message that is not theirs.

The `mentions` description of `send` and `reply` changes from "to put the message in the person's inbox" to say that `paul` notifies the person. Nothing else about the tools changes. The words are final only when the instruction is written; the point is the three things it says: use the field, the text does not count, and when.

## Tests

1. `Bus.Mentions` returns the unread messages an agent sent to the person, by a group mention and by a direct message, with the group and the sender, and not a group message that did not mention the person, not the person's own messages, and not a message already read or older than the look-back.
2. Mentioned by a chat of another process: a message written through a second bus handle, as the command line writes, is seen by the first.
3. The window makes one card for a mention with the title, body and fields above, makes no second card for the same message after another change to the bus, and makes none for a message the person has read.
4. Reading the message dismisses its card, and dismissing the card leaves the message unread.
5. The host notifier is told when the window is not active and Notify When Needed is on, and is not when the window has the focus or the setting is off.
6. The instructions say to mention `paul` in the field, that the text does not count, and when not to; and the tool's description no longer says "inbox".
7. A mutation check: with the detection looking at the wrong kind of delivery (`human_room`), test 1 fails; with the card's identity not made from the message, test 3 fails.

## DESIGN.md changes

- The paragraph on group chats says that a message that mentions the person puts a card in Notifications and, when the window is not active, a notification from the operating system, and that the chats are told to mention the person when they need him.
- The decisions table gets a row for the date.

## Questions for Architecture

1. Is a card for each message right to start with, with coalescing added only if bursts prove a nuisance?
2. Is a day the right look-back for cards after the window was closed?
3. Does Front End's card belong in the existing Needs You group, as I have assumed?

## Ruling

Architecture approved this shape on 29 October 2026 (message 631), with the look-back replaced.

1. **One card for each message,** as proposed. Coalescing waits until bursts prove a nuisance.
2. **No look-back by age.** A mention Paul has not read is a card however old it is, so a weekend away does not silently drop Friday's mentions; a message the bus has pruned no longer exists and cannot return. What the day was guarding against is the backlog on the first start of the build that adds cards, when every old unread delivery would become one at once. That is handled once: the first start records the highest message number it found, in the window's own state, and only mentions after it become cards. Older unread mentions stay in the inbox as they are today.
3. **The card belongs in Needs You,** since a mention is something an agent needs from the person.

## What was built

Built on 29 October 2026 (Dinah Engine Cleanup), the engine half. `Bus.Mentions(after)` returns the unread messages agents sent the person, by a mention or directly, `Bus.HighestMessage` gives the baseline and `Bus.MentionsRead` says which have been read. The bus watcher calls the window's pass each time the bus's change counter rises, and the pass makes a card for each new mention, dismisses the card of each read one, emits `notifications:added` with the card and `notifications:dismissed` with its identity so that the page need not read the store again, and tells the host notifier when the window is not the active one. The baseline is `mentionBaseline` in the window's settings, set on the first pass.

Two points the plan did not say. Reading a group's window now marks the mentions in it read in the inbox as well as the group's own unread count, since otherwise a mention the person had read in the group would stay unread in the inbox and its card would never clear. And the card's fields are `group`, `groupTitle` and `message` on the notification, with the sender's chat in `sessionTitle` and, when the chat is open in the window, its identity in `sessionId`; the sender's workspace is in `details`. The page half is Front End's: show a `mention` card in Needs You with Open Group, add and remove cards on the two events, and restore the cards from the store at start.
