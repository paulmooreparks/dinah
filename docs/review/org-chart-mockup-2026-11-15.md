# Org chart visual review

The static mockup is ready on `frontend/org-chart-mockup`, from master `adac3df7`. The source is `docs/mockups/org-chart.html` and `docs/mockups/org-chart.js`. It uses the shipped PUDL styles and the bundled Diagrams `yavcCanvas.render` function. It changes no product file or live chat.

The pictures show Platform and Delivery with their leads and members, Research folded, the Release Review cross-functional group and two free agents. Nina reports to Mara in Platform and belongs to the group led by Lena in Delivery. Reporting lines are solid; group memberships are dotted. Working, idle and waiting each have a glyph and a text label. Nina's open context menu includes the requested management actions and Delete Chat.

The four pictures are in `docs/mockups`:

- `org-chart-light-1440.png`
- `org-chart-dark-1440.png`
- `org-chart-light-390.png`
- `org-chart-dark-390.png`

The narrow proposal stacks teams and routes group links down outer gutters. It preserves readable box text. The open menu occupies a gap rather than covering another chat. This layout is a proposal for review; it does not implement a live chart, automatic layout, folding or management actions.

The capture ran through FIFO with an isolated headless Edge page for each picture. All four pages had no script errors, no horizontal overflow and a menu inside the viewport. Each picture contains four reporting branches and three group memberships. The drawing uses layout points to make four dotted segments in the wide view and five in the narrow view. The checks are saved in `docs/mockups/org-chart-checks.json`. Syntax and whitespace checks passed, and all four visual states were inspected.

The first capture rejected a fractional connector width in the invented drawing. The fixture now uses Diagrams' documented whole-number width. The first visual pass also exposed soft line breaks and links crossing box text; the final source uses separate text nodes and outer routing gutters. No assertion was weakened.

To reproduce the pictures from this checkout, set `NODE_PATH` to the main checkout's browser-test dependencies and run the following command with a new temporary output folder.

```powershell
./scripts/with-test-lock.ps1 node ./docs/mockups/org-chart-capture.cjs "$env:TEMP/org-chart-mockup-review-copy"
```

The chats are invented, and all controls are inert. No real home, remote service, credential, visible window, push, merge or installation was used. Team Lead should show these pictures to Paul before authorizing live implementation.
