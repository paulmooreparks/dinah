# Mentions in Group and Team Chats

Paul reported on 16 November 2026 that in a group or team chat he can only at-mention chats, and asked for the full set a one-on-one chat offers: documents, applets and other chats. Today a group's @ list (`bus-applets.js`) offers its member chats, @everyone and Add a Chat…, while a one-on-one chat's (`chat-mentions.js`) offers windows, bundled applets, open documents and open chats across workspaces.

The clickable mockup is `docs/mockups/group-mentions/index.html`, and opened as a plain file it starts on a group chat with the @ list open. It loads the shipped PUDL, brand, dinah, app and bus-applets stylesheets, the palette the real window applies, and the shipped `mention-list.js`, which draws both lists today. The proposed rules are in `group-mentions.css`. Pictures of every state at 1440 and 390 pixels, in light and dark, are in `pictures/`, and `explore/walkthrough.js` uses every part headless and checks it.

## The list

Typing @ in a group or team chat lists the group's members first, under the heading Members, Asked to Answer, with @everyone, as today. After them come the same groups a one-on-one chat offers, built the same way: Windows, Applets, Open Documents, and Other Chats, Referred To and Not Asked. A chat that is a member is listed once, as a member. Every reference's row says that it asks no one, and the line of keys at the foot of the list says that members are asked and everything else is a reference. Typing after the @ searches all of them, and Add… opens the same list.

## What a mention does

A mention of a member routes to that member and asks it to answer, exactly as today, and @everyone asks every member. Any other mention is a reference. It goes with the message as the one-on-one chat's references do (a document's resource link, an applet's verified identity, a window, a chat), and it wakes nobody. A chat outside the group is a reference too, so mentioning it never pulls it into the group's conversation.

The line under the box still shows the engine's answer about who the message reaches, which references never change, and adds how many references the message holds and that they ask no one.

## How a sent message shows the difference

A member asked to answer is the existing mention, bold and in the accent colour. A reference is plain text with its kind's glyph and a dotted underline, so the two differ by weight, colour and glyph together. Under the message, one line says who was asked to answer and another lists what was referred to and not asked, with a chat's workspace beside its name.

## Questions for Paul

1. Should Add a Chat…, which puts another chat into the group, stay at the end of the Members heading as it is today? I recommend yes, since it changes the group rather than the message.
2. Should a reference to a chat outside the group offer to add that chat to the group, for when Paul meant to ask it? I recommend no for now, because the line under the message already says it was not asked, and Add a Chat… is one step away.
