# Live organisation chart

The read-only Org Chart page implements Paul's approved proposal in [org-chart-view.md](../plans/org-chart-view.md). Its address, workspace filter, search and folded teams use the existing workbench URL state. The Window menu opens the page. Engine owns `OrgChart()` and `docs/design/org-chart-query.md`; the frozen query contract is commit `77559d93` on `engine/chat-cleanup`.

The page keys each chat by `chatId`. Its current `head` never determines identity. A serialized query reads the complete snapshot after local state, team, group, removal and restoration events. A query failure keeps the previous drawing visible with an error in a reserved status line. The page never discovers agent history or opens a chat.

Diagrams' bundled `yavcCanvas` draws the boxes and connectors. Positioned HTML inside those boxes provides accessible text, state badges, fold controls and menus. All chat boxes reserve 100 pixels. State, title and program text update inside those boxes without replacing the drawing, chat elements or layout. Long text has a tooltip and cannot grow a row. Only a changed set of canonical nodes or relationships, a filter, a fold action or a changed viewport width causes layout.

Connector waypoints follow clear horizontal and vertical corridors around the reserved boxes, with six pixels of clearance. Diagrams draws their segments using its documented point anchors and straight routes. These are the `anchor` node property and `route` edge property in the producer's `yavc.app/docs/canvas-extension.md`. The browser suite samples the rendered SVG paths against every chat, group and fold control. This catches a connection to a lower member that would otherwise cross an intervening member and suggest the wrong reporting line.

Reporting trees appear inside team frames. Nested leads remain in their parent tree, so each chat appears once. Folded descendants use the team's raised summary control as their visible group-link endpoint. Group members outside a team have their own section. Free agents have neither a team nor a group. Repair entries show the supplied reason and have no menu; unresolved relationships produce no inferred edges.

The narrow layout stacks team frames and uses fewer columns. Workspace and Find a Chat filter the logical chats, retaining the ancestors needed to explain a matching member. Group relationships stay dotted. Management entries are disabled and say "Coming next" in this stage.

Verification and the remaining host integration boundary are recorded in [the review note](../review/org-chart-live-2026-11-15.md).
