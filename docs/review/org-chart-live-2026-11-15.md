# Read-only Org Chart

The live page is being built on `frontend/org-chart-live`, based on master `26d67f03`. It follows the approved mockup and the query contract frozen at `77559d93` on Engine's cleanup branch. The implementation is generic in Dinah. Engine owns the App query; Release owns logical chat admission and resolution.

## Behaviour

The Window menu opens an addressable Org Chart page. Workspace, Find a Chat and folded teams use its workbench address. The page draws teams and nested reporting lines, dotted group links and free agents with Diagrams' bundled renderer. Chat boxes use canonical logical IDs. Repair entries show their reason and have no management menu. Other management actions are disabled with "Coming next".

State, title and program changes update the existing fixed-size boxes. They preserve the SVG, chat elements, keyboard focus and scroll position. A relationship change causes layout. Fold controls are raised buttons, and the narrow page uses fewer columns.

## Initial checks and corrections

The first browser run used FIFO ticket 2148 and completed both suites in 27 seconds. The wide frame assertions passed. The interaction suite stopped because its test clicked PUDL's hidden row-actions button before hovering over the row. The test now performs that pointer action. The narrow frame suite found a real minimum-width overflow; the chart now sizes itself to the available reading area. The original logs are in `%TEMP%/org-chart-live-first-20261115`.

The second browser run passed both width frame checks and all interaction checks except the menu's disabled-state assertion. PUDL uses `aria-disabled`, so the test now checks that documented state. Focused frontend policy, UTF-8 and asset checks passed under ticket 2154 in 0.471 seconds.

Four browser suites passed together under ticket 2160 in 29 seconds: `menus-through-updates`, `org-chart`, `org-chart-stability` and `window-commands`. The screenshots then exposed connector crossings that the original suite did not measure. A new path-intersection assertion is frozen with that source before the routing fix. Ticket 2165 was cancelled while queued to give Paul's document check priority. The page is not ready for handover yet.

## Integration boundary

These checks drive the real page in a hidden browser with invented OrgChart snapshots. They do not prove the pending host query, logical-chat admission or native runtime behaviour. The page calls no discovery or open method. A combined host candidate is required before product acceptance.

No visible window, real home, native agent history, credentials or remote service is used. Nothing is pushed, merged or installed by Front End.
