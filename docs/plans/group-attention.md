# Who a Group Message Reaches

## Conversational attention, approved 3 November 2026

Paul approved conversational attention for his group messages in message 3866. The lead settled the remaining cases in 3871. This section replaces the earlier no-mention rule for the person's messages, including the team-channel default to its lead. Agent messages retain their existing action-recipient, notice, team-relay and turn-budget rules. Implementation and independent review follow this record. Installation waits until after Paul's 15:00 demo unless he asks sooner, and never restarts the application automatically.

The engine reconstructs a group's engaged members from its own message history. A person explicitly mentioning a named member engages that member. An agent engages itself when it writes a text message in that group whose reply_to identifies the person's text message in the same group, or whose action recipients include the person. Naming the person in prose does not count. The existing team relay retains its meaning as explicit addressing to the person for this purpose, even when the lead becomes the effective recipient. Ordinary later agent posts do not count.

The engine orders engagement by descending message ID, using the stored mention order for members engaged by the same message. It keeps at most K members, default 3. Each expires T minutes after the timestamp of its last engagement, default 30; equality with the expiry is expired. K and T are persisted bus settings, shared by desktop, web and command-line senders. Their supported ranges are 1 to 100 members and 1 to 1440 minutes. There is no stored engaged set or in-memory source of truth. Restart reconstruction returns the same identities, order and expiry timestamps at the same clock time.

An explicit everyone reaches everyone and engages no individual. Named members alongside everyone are engaged. An inferred route never renews engagement. A member who leaves, is removed or is retired is excluded immediately; rejoining does not restore engagement from before that membership change. A closed chat which remains a member retains its engagement. Both an engaged route and automatic everyone can reopen closed member chats, just as an explicit mention does today.

At send time, a person's explicit recipients win. With none, the engine routes to the engaged set. With an empty set, it records everyone as an automatic action request. A group containing no eligible agent members still has no actual agent to wake; the recorded everyone intent is visible. Routing runs inside the transaction that writes the message and its deliveries, with the same fake-clock-capable engine clock used for expiry. Errors refuse the send rather than silently sending without recipients.

The existing mentions field keeps the normalized action recipients and everyone marker. New message fields keep the attention reason (explicit, engaged or everyone) and a JSON snapshot of the effective recipient identities and titles. The snapshot is the authority for what that send reached, even after membership or titles change. Legacy person messages with mentions but no attention reason are treated as explicit engagement evidence. Legacy unmentioned person messages do not engage anyone. Agent messages retain their original stored routing.

### Engine and composer contract

BusGroupReach(group, mentionIDs) returns a snapshot with group, version, reach, everyone, reason, engaged, expires and settings. The reach array contains the actual eligible member identities and titles, not an everyone pseudo-member; everyone is a separate boolean, including an explicit broadcast. Each engaged entry includes id, title, engagedAt and expires. The expires field is the nearest engaged expiry, or an empty string. The reason is explicit, engaged or everyone. The settings object contains limit and minutes. Explicit IDs use the same normalization and validation as sending, including typed mentions already bound by the shared composer helper.

The existing bus:changed event carries the bus counter. It invalidates every open group preview, covering replies, membership, title and setting changes. The preview's version is that counter from the same read transaction. This uses one existing event instead of maintaining a second group-event index. The page rejects stale requests with its request sequence and compares versions; time alone does not increment the counter. At the engine-provided nearest expiry, the page asks for a new preview. It never computes the new reach itself.

BusSendToGroup keeps its current arguments and returns the actual snapshot as BusMessage.reach. History messages return that same persisted snapshot. The page compares the returned reach with its displayed preview and reports a difference in one quiet line beneath the sent message. BusGroupAttentionSettings() and SetBusGroupAttentionSettings(limit, minutes) expose the two global settings to the person's Settings flow; agents cannot modify them through a bus tool.

With no engaged members, the placeholder says "Type @ to choose who answers, or send to everyone". Otherwise it names the engaged recipients and offers @ to choose others. A fixed-height text row says who the current message reaches, including everyone for an automatic broadcast. The polite live region announces only when that reach changes, after typing pauses. Neither updates nor expiry move existing content. The browser tests count announcements as well as checking the text and placeholder states.

### Verification and handoff

Engine guards cover engagement by explicit mention and by both forms of addressed reply, unrelated agent chatter, cross-group reply isolation, K ordering and bounds, T expiry with a fake clock, broadcasts with and without named recipients, automatic everyone, closed and removed members, membership rejoining, retired members, unchanged agent routing and restart equality including order and expiries. They verify persisted reasons and actual reach, validation failures and the transition between preview and send at expiry. Desktop bridge tests cover the preview and send result. Front End owns composer code and browser guards, and Quality independently reads both before Release integrates them.

Paul proposed on 30 October 2026 (message 811) that group chat work more like a person's own attention. A message that mentions "everyone" reaches every member, a message that mentions chats reaches those chats, and a message that mentions nobody gives each member a short notice of who sent it and what it is about, so the member can decide whether to read the whole message, answer it, or catch up later. This would reverse the rule Paul set on 29 October (message 660), under which his messages wake every member even with no mention. This document is Architecture's proposal for his decision. Nothing here is built.

## The Proposal

1. **A message that mentions "everyone"** is delivered whole to every member chat and wakes each one. "everyone" becomes a reserved mention that no chat can take as its title. The group's turn budgets still apply.
2. **A message that mentions chats** is delivered whole to those chats and wakes them. Every other member gets the short notice described next, as it does for a message with no mentions.
3. **A message that mentions nobody** gives every member a notice and wakes none of them. The notice is one line with the sender, a short summary and the message's number, and the whole message is one bus read away. The notice does not start a turn. It waits and comes in at the start of the member's next turn, together with any others, the way a badge waits for the person to look. Several notices are shown together, and a long run of them is cut to a count and the most recent few.

The third point is the one that changes what Paul experiences, and it matters for cost. Any delivery that starts a turn costs the receiving agent a whole turn with its whole context, even when it only decides to ignore the message. A chat with a large context that wakes to read one line and say nothing has spent most of what a full answer would cost. A notice that waits for the next turn costs a line, and so a group of five members stops paying five turns for every message that was really for one.

## Where the Judgement Agent Fits

The summary in the notice is a judgement call, in the sense of [judgement-agents.md](judgement-agents.md). The judge writes it from the message alone, and the rule behind it is the message's first line cut short.

Deciding who a message without mentions is really for is also a judgement, and the measurements in that document showed small local models getting it wrong. So I would not let a judge wake chats on its own. Instead, while Paul types a message with no mentions, the composer shows that it will reach the members as notices only, and offers the judge's suggested mentions beside it, which he can accept with one press. That keeps the decision with Paul and puts the suggestion where he is already looking, which is the never-interrupts rule. Without a judge, or when it is late, the composer shows the plain fact that nobody will be asked, with "everyone" offered as one press.

Messages from chats follow the same three rules. Today a chat's message with no mentions already wakes nobody, so for chats the change is that the members it does not mention now get a notice they can act on, instead of either the whole message or nothing.

## Questions for Paul

1. Should a message of yours that mentions nobody wake nobody, reversing your ruling of 29 October? I recommend yes, together with the composer showing that before you send.
2. Should "everyone" open closed members as well, as Engine assumed (message 812)? I recommend yes, because that is what the word says, with the group window showing that each one opened costs a session.
3. Should the notice begin with the message's first line cut short, leaving a written summary for the judge later, and should the judge's suggested mentions appear in the composer from the start? I recommend yes to both, since nothing happens unless Paul accepts a suggestion.

Front End and Engine added a guard (messages 814 and 815): a message that mentions a chat, or "everyone", always delivers its full text to that chat, and a test checks that a mention never becomes a notice. The engine makes the notice's one line once and sends it both to the member chats and, with the unread state, to the group window's unread marker, so the page does not cut it a second time. A judge's summary, when there is one, replaces that line later and is kept, so no send waits for a model.

Engine added two needs (message 812). The bus must be able to read one message by its number, which it will check and add if missing, and the run of waiting notices needs the limit described above.

## Paul's Decisions

Paul answered on 30 October 2026 (message 817). A message of his that mentions nobody wakes nobody, as the plan says, with no special case, which replaces his ruling of 29 October (message 660). "everyone" opens closed members too, within the background slots. The notice begins as the message's first line cut at 120 characters, and the judge's written summary comes later. The proposal above is accepted as written.

Engine builds the bus side: "everyone" as a reserved mention, notices that wait for the member's next turn with the limit on a long run, the one-line text sent with the unread state, the bus tool that reads one message by its number within the chat's own groups, and the test that a mention never becomes a notice. Front End builds the page side: the composer saying, before a message with no mentions is sent, that members will get notices only, with "everyone" offered as one press, and the sender and line in the group window's unread marker. The judge's suggested mentions in the composer wait for the judgement agents.

## Engine Side, Built

The bus side was built on 30 October 2026. Everyone is the mention that wakes all members and is kept beside the WakesAll mark; the person's messages no longer carry that mark. A message that does not wake a member reaches it as a Notice (`bus.Message.Notice`), written by `NoticeLine` as the first line of the text cut as a title is cut, and `FormatForAgent` shows the newest five per group and a count of earlier ones. The chat's history entry shows the same line. The `read_message` tool returns one group message by number to a member of that group, or to a chat that sent it or was sent it, and refuses direct messages. A group's overview gives `latest` (id, from, line) for the newest unread message, and leaves it out when nothing is unread. A chat title that is everyone is stored with " chat" added, so the mention can never be taken by a chat. Tests check that a mention is never a notice, that everyone opens a closed member, and that the person's unmentioned message wakes nobody.

## A Notice Wakes the Chat

Paul corrected the design on 30 October 2026 (messages 837 and 839). He meant a notice to reach the member at once, so that it can read the sender and line and decide then whether to read the whole message, answer, or catch up later, rather than wait for its next turn as built. A notice therefore wakes the chat it is for. The line, the tool that reads one message, the limit on a long run and the rule that a mention always delivers the whole text are unchanged.

Two guards come with it, because each wake is a turn over the chat's whole context. First, notices are gathered: a chat that is working, or that has already been woken for a notice whose turn has not ended, is not woken again, and the notices that arrive meanwhile are delivered together at its next turn. Second, a wake for a notice caused by another chat's message counts against the group's turn budget, so that chats posting without mentions cannot keep waking each other. Wakes for Paul's own messages are not budgeted, as he ruled for his messages in [group-turn-budgets.md](group-turn-budgets.md).

The wake was built on 30 October 2026. A notice rings the open member it is for through the doorbell. `Ring` steers a working chat only when `Bus.Asking` finds a message that asks it to answer, so a notice never interrupts a turn and the notices that come meanwhile are one delivery at the next turn. Delivering a notice from a chat spends a turn of the group's budget, and a message from a chat that mentions nobody wakes nobody once the budget is spent, which the sender's receipt says. Closed members are not opened for a notice. That is a choice made here and open to Paul's ruling, because opening a closed chat costs a session with its provider for a line the chat may ignore, and a closed member finds the notice waiting when it is opened.

The wake was built and installed the same night (merged as 42d889d). A notice rings only open members. A closed member is not opened for a notice, because opening it costs a session with its provider for a line it may ignore, and it finds its notices waiting when it next opens. A mention or "everyone" still opens closed members.

## Mentioning Documents, Windows and Applets in a Group

Paul asked on 30 October (message 975) to be able to mention windows, documents and applets, and possibly chats outside the group, from a group chat's composer, as he can in a single chat. It is not urgent, and it is tracked here until it is scheduled. A chat's composer already offers these through its mention picker, so the group composer can offer the same picker. What differs is that a group's members may be in other workspaces, so a reference to a document in one workspace must not give a chat in another workspace access to that folder. The likely shape is that a mentioned document travels as a bus attachment holding its content when the message was sent, which members read with the attachment tool they already have, and that a window, applet or chat outside the group travels as a description and a link the person can open, which grants nothing. Dinah UI designs the interaction, and Engine and Front End build it when Paul schedules it.
