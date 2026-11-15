# Org Chart of Chats

Paul asked on 15 November 2026 for a live org chart of his chats, of the kind a team member drew by hand on 13 November (`docs/org-chart-2026-11-13.canvas`, untracked). He wants to see team membership, reporting lines, looser associations and the chats that belong to nothing, and to manage them from the same surface: assign a chat to a team, create a team or a group, and fire a chat.

## Decisions

Paul answered three questions in this chat on 15 November 2026.

1. Dotted lines come from group-chat membership. There is no separate "also reports to" link to keep in step with anything.
2. A chat belongs to at most one team, as its lead or as a member, so reporting lines form a tree. A chat may belong to any number of groups. A team is a reporting line; a group is a loose association, such as a cross-functional group.
3. Dinah Front End draws a static mockup first, for Paul to react to before anything is built.

Paul also said on 15 November that the chart goes into the products built on Dinah as well. It is therefore built generically in Dinah, with no product's names or records in it, and reaches those products through their normal merge from Dinah.

## Shape

The chart is a Dinah page with its own address, under the Architectural Principles. It is not a diagram document. It uses the drawing and layout code of the Diagrams applet, and it reads Dinah's own records of teams, leads, members, groups and chat state, so it adds no new record of its own.

A chat's state (working, idle, waiting on a question) changes its box in place, and nothing moves when it does. The chart is laid out again only when the structure changes: a chat joins or leaves a team or a group, or a team is created or removed. Teams fold and unfold, and the page can be filtered by workspace, since Paul has many chats.

Solid lines are reporting lines within a team. Dotted lines are group memberships. Chats with no team and no group are shown together as free agents.

Each box has a context menu, with the same actions reachable from the keyboard: assign to a team, make lead, create a team, create a group, start a chat, and fire. Firing moves the chat to the Recycle Bin of the chat cleanup work, with its 30-day restore. Dragging a box onto a team is a later shortcut for assignment, never the only way.

## Order

1. Chat cleanup and the Recycle Bin, which firing depends on.
2. The ghost chats left behind by switching agent programs.
3. The mockup, then a read-only live chart.
4. The management actions on the chart.
5. Moving a set of chats, or a whole team, to another agent program at once, which Paul asked for as a later step.
