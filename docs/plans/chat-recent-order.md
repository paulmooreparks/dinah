# Recent activity in chat lists

Chats are ordered by their recorded updatedAt timestamp, newest first, within the existing Needs You, Working, and Chats groups. The shared Workspace chat list uses the same ordering. Combining multiple workspaces must not leave one workspace's older chats ahead of another workspace's newer chats.

Equal timestamps retain source order. Missing or invalid timestamps follow dated chats. Timestamps are parsed as instants so timezone offsets do not change their meaning. Opening a window alone does not invent a new activity time.

Live session updates reapply the order through the existing list renderer. Pointer movement, pressed buttons, keyboard focus, inline renaming, and open menus retain their existing movement protections. Rows and scroll positions are preserved through that renderer.

The focused browser fixture covers cross-workspace ordering, priority groups, equal and unavailable timestamps, live updates held by focus, and workspace and text filters. All focused Edge browser checks passed. Evidence is under build/chat-recent-order. The fixture uses the real frontend with a stubbed backend.

Installed as 0.0.0-compact-work-20261106 on 6 November 2026. The installed executable matches the build SHA256. A rollback backup and receipt are retained under build/compact-work-install/install-receipt.json. Dinah was not restarted; existing process IDs were unchanged.

The host now records live user and agent message activity in session updates, using its existing message journal as the durable source on reopen and list refresh. Replay and merely opening a chat do not advance this timestamp. Older or empty provider metadata cannot erase newer activity. The earlier fixture only verified sorting of supplied timestamps and missed absent live Codex timestamps.

Installed correction build 0.0.0-activity-corrections-20261106 on 6 November 2026. The compact-work, chat-recent-order, and message-boundaries browser suites passed; the full sessionhost suite and desktop journal integration tests passed. The collapsed screenshot confirms that commentary stays visible. The installed SHA256 matches the build, and existing processes were unchanged. The backup path and verification receipt are in build/activity-corrections/install-receipt.json. Restart remains manual.

