# Window stacking persistence proposal

Status: proposed for PUDL review; implementation and release are not yet authorized.

Dinah saves the window layout URL across restarts. The bundled PUDL window manager serializes open windows and the single top window, while keeping the remaining back-to-front order only in its private zOrder array. Restart reconstructs that order from window opening order, allowing formerly covered windows to obscure the chats the person was using.

PUDL should make the complete stacking order part of its documented layout state and URL serialization. A proposed z.order parameter lists open window keys from back to front, independently of taskbar order. PUDL owns validation, serialization and restoration. Dinah should persist that state through its existing layout URL, without inspecting private arrays or applying manual z-index overrides.

Duplicates and unknown keys must be ignored. Open windows omitted from an explicit order need a documented deterministic fallback. Old URLs without the parameter retain their existing behavior. The top window and parent-child window constraints must remain consistent with the resulting stack. Dock groups, minimized windows and taskbar order retain their own semantics.

Restoration must establish the saved stack before asynchronous applet loading can expose windows. Loading, retitling and reconnecting a background applet must not raise it. Explicit user activation still raises a window normally.

Acceptance requires saving three overlapping windows after several raises, restoring in a fresh page, and comparing the complete order and active window. Repeat with tiled windows overlapped by floating windows, minimized windows, parent-child windows, delayed applet initialization, dock groups, explicit taskbar order, and old or malformed URLs. Verify readable and minified distributions. Dinah then adopts the released assets in both vendor folders and runs its own restart regression.

Source evidence is in cmd/dinah-desktop/frontend/workbench.js saveLayout, ui-state.js restoreLayout, and bundled pudl-windows.js readURL, windowParams, stackOf and zOrder restoration. This establishes the missing serialization; the screenshots also warrant checking whether asynchronous startup changes the active window independently.

Submitted to PUDL Team Lead for contract review on 6 November 2026. The review request explicitly excludes implementation and publication authorization. Dinah retains ownership of adoption and restart verification after the upstream contract is accepted.


## Upstream review

PUDL Team Lead confirmed the missing serialization by source review on 6 November 2026. The review is recorded in C:/Users/paul/source/repos/pudl/docs/proposals/window-stack-persistence-review.md. Implementation and publication remain pending authorization.

The proposed public state exposes a detached state().zOrder array. Normalization operates on logical open keys even before fragments arrive. It removes invalid, duplicate and non-open entries, then prepends omitted open keys in open order below explicitly ordered survivors. A missing z.order parameter preserves legacy behavior. Explicit state must respect top and parent-child constraints without unminimizing windows. Children remain immediately above their parent, and edge docks retain their separate rendering band.

One normalization rule must serve parsing, candidate-state operations, rendering and serialization. The implementation audit includes close, rekey, replacement, sharing, default-layout detection, Back/Forward and pudl-regions.js recognition of z.order. Tests additionally cover normalization idempotence, region navigation, all-minimized state, failed fragment loading, and readable and minified distributions in all three browser engines.

Startup focus is a separate unproven cause. A restart regression must record initial restored order and top, subsequent window-change snapshots, focus targets and delayed applet readiness. It must include the person activating a different window during loading. Dinah's editor-focus route may explicitly raise an editor, but this review did not establish that startup invokes it. Serialization alone cannot be claimed to fix later host activation.

This public state and URL addition requires a minor PUDL release under the upstream release policy. The reviewer suggested 0.100.0 if 0.99.x remains current when release work starts. Taskbar pinning is outside this proposal. No runtime reproduction or upstream implementation was performed by the reviewer.

## Dinah integration readiness

The 0.100.0 adoption is on hold at the person's request while upstream prepares 0.101.0. The published 0.100.0 tag was verified, but no bundled assets or installed executables were changed for that adoption.

Integration will use an exact candidate commit from C:/Users/paul/source/repos/pudl-window-stack in an isolated fixture, preserving the current installed build. The bounded restart fixture will exercise Dinah's saved layout path with overlapping chat and editor windows, several explicit raises, and a distinct taskbar order. It will compare public stack/top and rendered overlap before restart, immediately after restoration, and after delayed editor/chat initialization. It will also cover a user activating a different window while an editor is still loading. Evidence will record window-change snapshots, focus targets, editor-focus events and readiness milestones. The focused host suites cover workbench, editor and persisted UI state. No desktop installation or publication is authorized by the upstream review request.

Source inspection found that editor-applet.js dispatches dinah:editor-focus only through its explicit open(v, intent) helper. The restored editor instead enters registered init and ed.ready, which loads the draft, creates views, sets mode and restores position. That initialization block does not directly call focusEditing. The explicit focus event schedules focusEditing after readiness, and focusEditing unconditionally raises the editor unless embedded. This makes a delayed explicit open a concrete focus-stealing risk if the person activates another window before loading completes. It does not establish that ordinary restart dispatches the event. Runtime restart evidence is still required before attributing the observed startup stack to this route.

## Candidate integration result, 6 November 2026

Dinah tested frozen PUDL tip 09bb8ab1a960ceb5df500b7024a5d12692dc5589 from C:/Users/paul/source/repos/pudl-window-stack. The four readable asset hashes matched the supplied manifest. The fixture serves those four files, or their corresponding dist/min files, through request overrides. It does not modify either vendor directory.

The bounded Edge test passed for readable and minified assets. Each variant saved one editor and two overlapping chat windows after repeated raises through Dinah's real workbench and UI-state code. Two fresh browser contexts then restored the native-state fixture: one with delayed editor/chat readiness, and one with an intervening activation through PUDL's public raise API. All four restorations preserved the full public zOrder, top and the window hit at the overlapping screen point. Delayed readiness preserved the selected order and overlap. No dinah:editor-focus event occurred during ordinary restoration. The test records window-change and focus events, snapshots and screenshots.

This is a headless Edge frontend integration with the native bridge stubbed and its saved state retained across fresh contexts. It is not a native executable restart or a claim about every startup path. It does not cover a physical taskbar click, Linux CI, or the complete upstream browser matrix. Explicit delayed editor-open remains a separate source-identified host activation risk; this test did not invoke that route or prove that ordinary restart uses it.

The fixture is cmd/dinah-desktop/frontend/e2e/window-stack-candidate.js. Passing evidence is build/window-stack-candidate-verified2/stack-evidence.json and the four adjacent screenshots. Earlier fixture failures are retained in the preceding window-stack-candidate output directories. They involved waiting for a hidden editor to become visible, an invalid synthetic pointer event, and incorrect minified filenames. The corrected run passed in 14 seconds with no page errors. No PUDL assets were installed or published. Upstream release ownership remains with PUDL Team Lead.

## Released distribution adoption, 6 November 2026

At the person's request, Dinah now bundles the full PUDL 0.101.0 distribution in both desktop and web asset folders. GitHub's latest release was checked at adoption time. The release commit is 26e02cdc17bfdff2dfb2b99074590eb0a5fc8beb. The downloaded archive SHA256 is 3ccc87bd0eb5a882b982c4506d9cf1897ce43d22abdadd47d35a1ccf476ee791. All 193 distribution files were compared byte for byte with the tag and verified in both vendor folders. Previous vendor directories remain under build/pudl-01010-release.

The existing restart fixture now tests the bundled released assets rather than an upstream worktree. The six suites compact-work, editor, tray-glyphs, ui-state, window-stack-candidate and workbench passed in build/pudl-01010-tests. The bounded restart coverage remains a frontend test with a stubbed native bridge; actual desktop restart is left to the person.

The desktop and web Go packages passed (170.246 seconds and 9.292 seconds). The typography regression passed and its screenshot was inspected. Build 0.0.0-pudl-01010-20261106 was installed without restart. Installed SHA256 is 5F92BE1B0521962E16DC98AF5150C124670F1D131D51A640EB92F6E054B1C4B5. The prior executable remains at C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-pudl-01010-20261106-210541. The receipt is build/pudl-01010-release/install-receipt.json. Process IDs were unchanged. Existing layouts saved by the old runtime cannot recover a full stack that was never serialized; after starting this build, arranging windows records the new stack for subsequent restarts.
