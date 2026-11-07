# List rendering investigation

Paul requested a broader investigation after the chat row cache improved responsiveness and he observed faster startup. This audit examines unchanged refreshes across other views without deleting or limiting stored items.

## Findings

The waste is repeated content reconstruction in individual renderers. It occurs in panes, popovers and file trees. Sidebar placement alone does not explain it. The shared reconciliation helper preserves outer nodes, but callers can still replace every descendant during its update callback.

The desktop frontend was exercised in headless Edge with synthetic catalogs of 10, 150 and 500 items. Each sample set contains five unchanged refreshes after initial rendering. These timings measure JavaScript execution, excluding subsequent browser layout and paint. Team samples include event dispatch and cloning the fixture; Group Chats samples time its scheduled draw callback. These are diagnostic measurements, not startup timings or timing gates.

| View | Items | Median JavaScript ms | Maximum ms | DOM mutation records per refresh |
| --- | ---: | ---: | ---: | ---: |
| Group Chats | 10 | 0.5 | 0.7 | 19 |
| Group Chats | 150 | 2.5 | 2.7 | 159 |
| Group Chats | 500 | 6.2 | 8.8 | 509 |
| Team Report | 10 | 1.0 | 1.2 | 167 |
| Team Report | 150 | 9.7 | 11.8 | 2,407 |
| Team Report | 500 | 34.7 | 41.6 | 8,007 |
| Notifications | 10 | 0.1 | 0.1 | 0 |
| Notifications | 150 | 0.2 | 0.3 | 0 |
| Notifications | 500 | 0.4 | 0.4 | 0 |

Mutation records count operations, not all removed or newly constructed descendants. Group Chats removes the old container content and appends newly built rows, so its smaller record count does not establish a smaller layout cost.

### Notifications

`notices.js` retains cards by record version and skips rebuilding unchanged records. Direct pane refresh confirmed zero mutations at all three sizes. Its mounting mechanism batches synchronous store changes into a microtask. This result does not establish that every producer avoids redundant updates; delivery restoration explicitly replaces some existing notices.

### Team views

`teams.js` retains member rows but replaces state badges on each update. The detailed report also rewrites reason, metadata and usage text and attributes. Its report subscription exists only while the popover is open. The lead's member menu uses another renderer with the same badge replacement pattern, and that renderer runs for accepted team snapshots even while the menu is closed. Ordinary group member menus in `bus-applets.js` also replace badges. Team Report was measured directly; the other two paths are source findings.

### Group Chats

`bus-applets.js` subscribes its pane to bus and execution-state changes. Its render function clears `.bus-results` and recreates the sections and rows. The audit opened this pane with synthetic group rows, dispatched execution-state events without changing the catalog and measured its scheduled render. This demonstrates unnecessary redraw work independent of chat-list rendering.

### Workspace sources and Files

`workspace-browser.js` clears the visible source tree in `drawTree`, reconstructs all rows, enhances the tree and manually restores focus. Folder model notifications schedule a redraw, while loading and explicit toggle paths can also request direct redraws. Its workspace setter already avoids work when source definitions are unchanged. A change in a loaded folder can nevertheless reconstruct unrelated visible branches.

The vendored `parks-files/filebrowser.js` replaces tree and list HTML in its render methods. Its resource model change listener calls refresh. These are source-confirmed reconstruction paths. Their scaling and layout costs have not yet been measured, and this report does not rank them by timing.

## Latest desktop capture

Paul supplied a roughly 28-second capture after the chat fix. It contains eight long tasks totaling 573 ms, with a maximum of 87 ms, seven long frames with a maximum of 156.6 ms, and a maximum recorded input delay of 81.8 ms. Four frames report forced style/layout of 52.7, 104.2, 65.8 and 69.7 ms. Recorded backend emit and enqueue counts are zero. The raw input ring reports 205 wrap omissions; the retained slot aggregates still report their own counts and maxima.

This capture is encouraging but does not identify which renderer caused those frames. Its interaction sequence and duration differ from the earlier drag and resize captures. No causal startup comparison was performed.

## Decisions and next work

The existing Notifications version cache demonstrates that a replacement list framework is unnecessary. Subsequent fixes should make each affected renderer preserve unchanged row content, retain current event data for actions, and update only affected grouping and counts. Closed popovers should avoid rendering work where their lifecycle permits it, then reconcile current data when opened.

The first implementation candidates are Team member badges and Group Chats reconciliation. Workspace source and Files reconstruction need focused runtime measurements before choosing their update strategy. Any changes must preserve keyboard focus, expansion, selection, filtering and complete access to stored items.

PUDL Team Lead completed an independent investigation using the bundled assets in Edge 156.0.4314.42. Its report and reproducible harness are in `C:/Users/paul/source/repos/pudl/tests/output/list-layout-investigation-20261104.md`. Each isolated action ran seven times with 500 rows.

Replacing one row label with identical text triggered 1,000 tabindex writes across two lists, including 500 in the untouched list. Repeated tree enhancement generated 1,502 mutations; one Down key generated 2,002. A native select option label change rebuilt all 500 generated rows. The rich list-box fixture generated only two mutations, although its selection pass still traversed all rows. These measurements establish shared amplification. Their roughly 0.8 to 3.2 ms elapsed times through timer delivery do not explain the captured long layout stalls, and native-select findings must not be attributed to Dinah views without confirming their control type.

The recommended PUDL correction is conditional attribute writes and dirty work scoped to the affected list, plus mutation-free unchanged tree enhancement and tab-stop changes limited to affected items. Dynamic insertion, focus, expansion and accessibility require cross-engine checks. Toolbar folding, master-detail resize refresh, header status fitting and repeated window retitling remain geometry candidates requiring trace attribution. The investigation found no generic observer that refreshes every window for arbitrary deep list mutations.

Paul subsequently prioritized correcting the tray's glyph-first compaction before other fixes or installation. That correction is recorded in `tray-glyph-priority-2026-11-04.md`; it does not resolve the list findings above. No list optimization, PUDL release or installation follows automatically from the independent report.

## Evidence and scope

The diagnostic suite is `cmd/dinah-desktop/frontend/e2e/list-render-audit.js`. The complete run passed and retained its measurements in `build/list-render-audit-groups/list-render-audit.json`. Earlier narrower runs remain in `build/list-render-audit` and `build/list-render-audit-expanded`. All used synthetic data and headless protocol input. No live chats were changed.

This investigation adds a diagnostic suite and this report. It changes no production renderer and does not build, install, commit or publish an update.

## Measurement build implementation

Paul authorized a coordinated build on 4 November. The workspace tree now compares visible presentation state before replacing its DOM. The signature includes expanded descendants, loading/error/pagination state, permissions, selection, source definitions and entry identity. Entry identity prevents retained event handlers from referring to replaced model objects. Unchanged updates retain focus and tree nodes; changed presentation still uses the existing reconstruction path.

Five synthetic samples at each size measured workspace redraw medians of 1.3, 9 and 26 ms before the correction, with 120, 1660 and 5510 mutations for 10, 150 and 500 entries. The integrated correction measured 0, 0.1 and 0.3 ms, with zero mutations at every size. These are synchronous renderer timings, excluding subsequent layout and paint. Evidence is in `build/workspace-before/workspace-render-audit.json` and `build/measured-fixes/workspace-render-audit.json`.

Group Chats now skips unchanged presentation, including derived live state and formatted timestamps. Team member rows retain fresh action data while updating badges and details only when their presentation changes. Budget content also avoids equivalent redraws. The first integrated measurement reduced 500-row Group Chats median draw time from 6.2 to 0.4 ms and mutations from 509 to zero. Team Report fell from 34.7 to 4.2 ms and 8007 to three mutations. The remaining three visibility writes were then made conditional and a zero-write assertion was added for all audit surfaces.

PUDL commit `07f25584e53221ab270750ea66bbefa081a80e09` supplies only master-detail and tree scripts, minified equivalents and source maps. Every file was verified against the delivered manifest before copying into both Dinah asset trees. PUDL Team Lead reports all 30 focused readable/minified runs passing across Chromium, Firefox and WebKit. The local glyph-first window patch remains intact. Toolbar geometry remains a separate investigation; these changes do not establish the cause of the intermittent live long frames.

The six focused desktop suites passed in 40 seconds: workspace rendering, workspace sources, list rendering, Teams, Group Chats commands and tray glyphs. The workspace suite checks existing lazy loading, paging, menus, selection, persistence, recovery and source behavior. The new audit checks zero writes, focus identity and changed labels at each catalog size. No stored chats were removed or changed.

The final zero-write regression run passed all three suites (Chats scaling, list audit and Teams) in 21 seconds. At 500 rows, Team Report measured a 3.9 ms median with zero mutations. The production Windows build is build/measured-lists-20261104/dinah-desktop.exe, version 0.0.0-measured-lists-20261104, SHA-256 d3b1f303971e49bf070a97a8d86b28fdc0033c6e0386b5215001f57d5161d12c. Its manifest records provenance and limitations. This checkpoint is built but not installed.

Paul requested PUDL 0.97.11 and installation. Both vendor folders now contain all 187 tagged distribution files, verified byte-for-byte by SHA-256. The immutable release commit is cc8f8f1ac723eefe3c2b4ace727442bcf6996d97. The release supersedes all local PUDL overlays and includes unchanged-toolbar-label suppression and the compact-first tray policy. Installation checks cover Editor, tray glyphs, workspace sources and rendering, list rendering, Chats scaling, composer folding and the workbench.

Installation completed at 2026-11-04T20:47:50.9189497+08:00. All eight desktop suites passed in 113 seconds, desktop Go tests passed in 113.999 seconds, and the production build succeeded. Installed C:/Users/paul/.dinah/bin/dinah-desktop.exe with SHA-256 189f6ac6f4b91b489706b85adfad33101cfffe7b094be8c90918ac6a6377fdd7. Rollback is C:/Users/paul/.dinah/bin/dinah-desktop.exe.old-pudl-09711-20261104-204750, verified against the previous installed executable hash. Dinah was not restarted. The editor titlebar chip and measured list fixes are included.
