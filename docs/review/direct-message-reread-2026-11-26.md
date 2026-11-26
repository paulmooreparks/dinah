# Direct-message rereading

An agent could receive a direct message but could not reread it by number because `ReadMessage` rejected every non-group message. The repair permits text-message reads by the sender or an authorized recipient using the existing `mayRead` predicate. Group membership and group correspondence access retain their existing checks. Other message kinds remain outside this tool's contract.

The MCP description now includes direct messages and explains that reading does not consume a pending delivery. There is no page or delivery-rule change.

The updated correspondence test exercises sender and recipient reads before and after delivery, refusal of an unrelated chat in the same workspace, unchanged delivery counts and unchanged bus counters. The focused regression passed. The locked, uncached Go gate for `internal/bus`, `internal/mcpserver` and `internal/profile` passed with exit zero. No browser suites were run because this change has no page files or rendered behavior.

The diagnostic branch is separate and is not part of this change. Integration, the full Go gate and installed-program verification remain the integrator's next steps. No merge, push or install was performed on this branch.
