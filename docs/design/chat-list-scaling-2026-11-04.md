# Chat sidebar redraw scaling

Paul asked whether the sidebar's 99+ chat count contributed to sluggishness and authorized investigation and correction without removing chats. The badge abbreviates the count; it does not cap the catalog or indicate that every chat has an active agent.

The sidebar reused row containers but rewrote their text and attributes and recreated status and attention badges on every redraw. A status badge includes hidden reserve chips for stable geometry, so replacing it multiplies the DOM work. A chat-state change or team-state event can redraw the whole list. The subsequent scroll-position reads also require the browser to resolve pending layout. These costs grow with the catalog.

The change caches the displayed values of each row, including title, formatted timestamp, provider, workspace label, live chat state and its description, attention count, earlier subagent count and selection. A row with unchanged displayed values retains its existing DOM. Group counts and classes are only written when changed. Grouping and pointer/focus holds still run, and every matching chat remains available. This change does not virtualize, truncate, delete or hide chat history.

The benchmark opens the actual desktop frontend in headless Edge with synthetic catalogs of 10, 150 and 500 closed chats. It dispatches eight team-state events without changing the catalog and measures the sidebar's scheduled draw callback and DOM mutations within the list. The baseline serves the pre-change `chat-browser.js` from HEAD in the same current frontend. The result demonstrates a scaling contribution; it does not attribute every earlier native drag/resize stall to this code or measure end-to-end input latency.

| Chats | Before median draw | After median draw | Before mutations per draw | After mutations per draw |
| --- | --- | --- | --- | --- |
| 10 | 6.25 ms | 0.25 ms | 137 | 0 |
| 150 | 84.00 ms | 0.95 ms | 1,957 | 0 |
| 500 | 292.45 ms | 2.90 ms | 6,507 | 0 |

Evidence is retained under `build/chat-list-baseline-verified` and `build/chat-list-verified`. The first baseline run recorded measurements but failed while saving its artifact because the harness used the wrong command-line argument. The corrected baseline run passed. An added provider-change assertion initially lacked the Claude provider in its fixture; the corrected fixture passed with the product code unchanged.

The six focused suites passed in 118 seconds: chat-list-scaling, chat-states, chats-indent, list-pointer-hold, list-state-chip and list-state-stability. The final scaling suite also checks changed titles, providers, execution states and reasons, stable badge identity for unrelated rows, and complete restoration after clearing a filter. It passed in 11 seconds. `git diff --check` passed.

The existing desktop and Git follow-up changes remain intact. Native desktop input and restart are outside this verification.

The Windows build `0.0.0-chat-list-20261104` was installed at `C:/Users/paul/.dinah/bin/dinah-desktop.exe` at 19:40:36 Singapore time. Its SHA256 is `76367117693e09c4b282cfcec7c313325e205b79b72219d7c06004be1704111d`. The previous desktop remains at `C:/Users/paul/.dinah/bin/dinah-desktop.exe.old-chat-list-20261104-194036`, SHA256 `1af96ee49a7e095fe100315750a3647210d6091da2e3d36fc15cca15878ab2c3`. Build, staged, installed and rollback hashes were verified. The embedded sidebar, index, Editor stylesheet and PUDL window script match the source bytes. The running desktop was left open; activation requires Restart to Update.
