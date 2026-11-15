# Read-only Org Chart

The UI candidate is ready on `frontend/org-chart-live`, based on master `26d67f03`. Its product source is `7a265d9425141fa253dd4212629e988774962678`, following the initial source and checks at `e2cae3d49ea04a962f32ddf3715037d3629938c6`. It follows the approved mockup and the query contract frozen at `77559d93` on Engine's cleanup branch. The implementation is generic in Dinah. Engine owns the App query; Release owns logical chat admission and resolution.

## Behaviour

The Window menu opens an addressable Org Chart page. Workspace, Find a Chat and folded teams use its workbench address. The page draws teams and nested reporting lines, dotted group links and free agents with Diagrams' bundled renderer. Chat boxes use canonical logical IDs. Repair entries show their reason and have no management menu. Other management actions are disabled with "Coming next".

State, title and program changes update the existing fixed-size boxes. They preserve the SVG, chat elements, keyboard focus and scroll position. A relationship change causes layout. Fold controls are raised buttons, and the narrow page uses fewer columns.

## Initial checks and corrections

The first browser run used FIFO ticket 2148 and completed both suites in 27 seconds. The wide frame assertions passed. The interaction suite stopped because its test clicked PUDL's hidden row-actions button before hovering over the row. The test now performs that pointer action. The narrow frame suite found a real minimum-width overflow; the chart now sizes itself to the available reading area. The original logs are in `%TEMP%/org-chart-live-first-20261115`.

The second browser run passed both width frame checks and all interaction checks except the menu's disabled-state assertion. PUDL uses `aria-disabled`, so the test now checks that documented state. Focused frontend policy, UTF-8 and asset checks passed under ticket 2154 in 0.471 seconds.

Four browser suites passed together under ticket 2160 in 29 seconds: `menus-through-updates`, `org-chart`, `org-chart-stability` and `window-commands`. The screenshots then exposed connector crossings that the original suite did not measure. The new path-intersection assertion was frozen with `e2cae3d4` before the routing fix. Ticket 2165 was cancelled while queued to give Paul's document check priority.

An isolated, unchanged checkout of `e2cae3d4` reproduced all four intersection failures in six seconds. The wide chart crossed another chat and a nested fold control. The narrow chart crossed intervening members. The same assertions passed after the corridor routing fix, with no changed threshold. [The failing log](org-chart-live-2026-11-15/connectors-before.txt) is retained.

## Review corrections and final verification

The review requested quieter team boundaries, readable group destinations and automatic refresh. Team panels now have quiet solid borders. Dotted group routes use gutters and never run along a container edge. Labelled group boxes appear after the relevant teams rather than below unrelated teams. Every member route ends at its group box. Retry Loading appears only after a failed read, in a reserved toolbar slot.

All four browser suites passed together under FIFO ticket 2185 in 46 seconds: `menus-through-updates`, `org-chart`, `org-chart-stability` and `window-commands`. Both chart suites passed again on the final source in 13 seconds after a private route-cache key correction. The assertions check container-edge clearance and every member's labelled endpoint in light and dark at 1440 and 390 pixels. They also check error-only retry, URL reload, filters, nested membership, repair entries, folding, keyboard menus, serialized queries and removal-triggered refresh without history discovery.

The frame suite records 104 samples across both widths while execution state, long titles, long program names and explanations change. It verifies fixed box, canvas, scroll and taskbar geometry, retained SVG and chat elements, and retained keyboard focus.

Focused frontend ownership, UTF-8 and release-asset checks passed under FIFO ticket 2189 in 0.395 seconds. The subsequent source change only gives the private route-cache key an unambiguous tuple encoding. Whitespace checks passed.

The screenshots and frame measurements are in [the evidence directory](org-chart-live-2026-11-15/). All eight screenshots were visually inspected. The main view and group area are captured in both themes at both widths. [The browser log](org-chart-live-2026-11-15/browser-checks.txt) records every assertion.

The final chart command was `pwsh ./cmd/dinah-desktop/frontend/e2e/run.ps1 -Only org-chart,org-chart-stability -Jobs 1 -Out $env:TEMP/org-chart-live-final-20261115`. The four-suite run used `-Only menus-through-updates,org-chart,org-chart-stability,window-commands` and output folder `%TEMP%/org-chart-live-gutters-20261115`. The focused Go command was `pwsh ./scripts/with-test-lock.ps1 go test ./cmd/dinah-desktop -run 'Test(FrontendJobsStayInTheirHomes|FrontendRulesFindTheirHomes|ReleaseAssetsExcludeLocalFixtures|ThePagesTextFilesAreUTF8)$' -count=1`.

## Integration boundary

These checks drive the real page in a hidden browser with invented OrgChart snapshots. They do not prove the pending host query, logical-chat admission or native runtime behaviour. The page calls no discovery or open method. Engine must combine its query and Release's admission adapter with this UI before product acceptance. The chart remains read-only; the Thinking control is the next direct assignment, followed by importing conversations.

No visible window, real home, native agent history, credentials or remote service is used. Nothing is pushed, merged or installed by Front End.

## Second routing review

The next review found a real crossing through the Platform panel and a shared run between Cal and Nina's groups. The stronger rendered assertion reproduced both faults in light and dark at 1440 pixels in an eight-second run against the previous product source. The previous edge-only guard missed them. Its failure log is retained as interiors-before.txt.

The pending correction permits only a member's direct horizontal exit inside its own panel. Every later segment treats all panels as obstacles. Each group reserves its gutter runs against the other groups. A member with a neighbour to its right exits left, and a member in several groups gets separate ports. The renderer's documented integer point-anchor constraint also applies to these ports. Both chart suites passed with the stronger assertions in nine seconds, then in eleven seconds after retaining two-column member rows at the wide width. The final run passed in ten seconds under ticket 2224 after a status-chip clipping correction. Its 104 frame samples remain stable. Focused frontend, UTF-8 and release-asset checks passed on the final source in 0.389 seconds under ticket 2225. The final command used -Only org-chart,org-chart-stability and output folder %TEMP%/org-chart-interiors-final2-20261115. The evidence directory now contains these final pictures, measurements and logs. The earlier four-suite run above predates these routing corrections.
