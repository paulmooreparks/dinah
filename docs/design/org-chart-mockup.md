# Org chart mockup

The mockup implements the visual proposal in `docs/plans/org-chart-view.md`, with invented chats only. It adds no live chart or management command. The architectural source remains `C:/Users/paul/OneDrive/Documents/Architectural Principles.md`.

The page uses the shipped PUDL light and dark palettes, type, controls and menu treatment. The canvas uses the bundled Diagrams JSON Canvas renderer, including its actual box and connector drawing. The palette comes from PUDL's surface, text, border, accent and status tokens. There is no separate mockup colour system. PUDL's font and normal control scale keep the chrome consistent with the application.

Two expanded teams show a lead above two members, with solid reporting lines. A cross-functional Release Review group connects a reviewer from Platform with the Delivery lead and tester through dotted lines. Research is folded into a compact summary. Two free agents have no connectors. Each visible chat has a text state and a glyph with the matching PUDL status colour. The reviewer is selected and has an open context menu.

The wide composition places the two expanded teams side by side, with associations below them and free agents at the side. A gap between teams leaves room for the open context menu. The narrow composition stacks the teams and the association, preserving readable text rather than shrinking the whole drawing. Its open menu occupies the gap between teams. The folded team and free agents remain visible in the full-page picture. This is a design proposal for narrow layout, not implemented automatic layout behavior.

Group connectors follow outer gutters so they do not cross chat text. Invisible drawing points form those bends in the static fixture. They represent layout only and add no relationship or application record. Three group memberships use four drawn segments in the wide view and five in the narrow view.

The context menu proposes assigning a team, making a lead, creating a team or group, starting a chat and deleting the selected chat. Delete Chat refers to the cleanup Recycle Bin. All controls are inert in this static mockup. The four requested pictures are captured from the same source at 1440 and 390 pixels in both colour modes.
