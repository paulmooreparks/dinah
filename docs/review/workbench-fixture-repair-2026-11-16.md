# Workbench fixture repair

The workbench suite had two stale fixture contracts on the shipped narrow-window base. Its launcher list omitted Decisions, Clean Up Chats and Org Chart. Its SavedChats fallback exposed only the current workspace, so switching to all workspaces could never show the second seeded chat. The Skills menu checks already passed.

The fixture now supplies both workspaces through SavedChats and expects the complete current launcher catalog. Product code is unchanged. The workbench, workbench-narrow and chat-inventory suites pass together in 13 seconds. The earlier failures and the catalog observation remain in private temporary output. This verifies the invented browser fixture; it does not claim installed-window acceptance.
