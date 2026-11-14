# Conversation context in Decisions

The reading pane reserves a fixed-height conversation area for a group request or mention. It reads at most five messages strictly before the carrying message, oldest first, through BusGroupMessageContext(group, messageId). Engine owns that query and its tests. The query validates the target's group and changes no read state. Direct requests explain that they have no group conversation.

Each message shows its sender, time and two lines of plain text. An explicit expansion stays inside the area's scrolling bounds. Each message links to its position in the group. Loading, empty results, errors and retries use the same bounds. A result is applied only to its still-connected area, so changing selection cannot show a previous request's conversation. Reading context never calls a mark-read method; opening a group follows its existing navigation behaviour.

Membership events use the bus's readable line. A message without text directs the person to open it for its contents rather than displaying an empty entry. The frontend validates the count, group, ordering, times and upper message bound before drawing any returned entry.

The answer controls and delivery protocol remain unchanged. The architectural principles remain at C:\Users\paul\OneDrive\Documents\Architectural Principles.md.
