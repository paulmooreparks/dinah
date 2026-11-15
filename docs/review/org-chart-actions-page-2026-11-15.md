# Org chart management page progress

The frontend work is on frontend/org-chart-actions from master 317d2c3d, combined with Engine's immutable host candidate c3cf3013. The approved host contract is docs/design/org-chart-actions.md. The host retains ownership of its wrappers and tests; this branch adds presentation only.

The chart keeps the existing PUDL palette and drawing. A legal action opens an addressable form in the chart window, with a Back to Chart control. The form uses raised controls, a flat explanation and a reserved result region. It retains the chart's workspace, text filter and folded teams. Menu loading uses PUDL's owned context update API so a late query cannot replace somebody else's menu. Keyboard and row-menu entry use the same host query.

The form reads eligible leads separately from member choices. Submission sends canonical chat identities and the host token, never a title-derived native identity. A partial Create Team result disables another creation and offers only the returned recovery targets. Conversion reuses the returned group. A lost result disables automatic repetition and asks the person to inspect the chart and groups. Deletion uses the host confirmation, and explicit Start a Chat opens the returned chat through the ordinary window path without another creation call.

The lane spacing change reserves sixteen pixels between parallel runs from different groups. The rendered assertion will measure that spacing alongside the existing panel-interior, border and endpoint checks. Geometry depends on structure and filters, while execution-state changes continue to update boxes in place.

The first routing and unchanged frame checks are queued. The management fixture suite is drafted separately while those run. No rendered management acceptance or combined hidden action check is claimed yet. Import from Codex remains separately unmerged at 435d1226 pending Release's host gates.
