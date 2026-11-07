# Pinned browser filters

Filtering controls at the top of browser applets must remain visible while results scroll. This applies in side docks and floating windows.

The shared browser toolbar and Notifications filter row now use sticky positioning with an opaque theme surface. Notifications delegates scrolling to its applet container so an unconstrained inner overflow container cannot defeat sticky positioning. Workspace retains its existing pinned toolbar and Pins region. Group Chats and Skills retain their separately scrolling results.

The focused Edge browser fixture checks Chats, Workspace, Notifications, In Progress, Group Chats, and Skills with overflowing content in both docked and floating layouts. All twelve checks passed. Evidence is under build/pinned-filters-modes. The fixture uses the real frontend with a stubbed backend; this is not verification in the installed desktop application.

Installed as 0.0.0-compact-work-20261106 on 6 November 2026. The installed executable matches the build SHA256. A rollback backup and receipt are retained under build/compact-work-install/install-receipt.json. Dinah was not restarted; existing process IDs were unchanged.
