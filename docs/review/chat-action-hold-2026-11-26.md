# Chat list action hold

Paul approved this change with "Yes, approved. Go ahead with it as described." Software Director relayed that approval in message 12012, and Dinah Team Lead assigned the implementation and direct Integrator handover in message 12013. The branch is `frontend/chat-action-hold`, based on master fa4b7165.

Focus alone no longer holds chat rows in their previous groups. A key press renews a two-second hold, alongside the existing pointer-motion, pressed-pointer and open-menu holds. The existing redraw restores the same row's focus with preventScroll and preserves ancestor scroll positions. Labels, group counts and the tab's busy count are unchanged. The layout plan records the new rule in both places that described the earlier hold.

## Verification

The focused six-suite selection ran hidden and headless. Chat states, Chats busy count, list-action-hold, list-pointer-hold and list-row-layout passed in the initial 125-second batch. The new regression covers light and dark at 1440 and 390 pixels, retained focus, a nonzero unchanged scroll position, renewed key and pointer holds, and open row menus. Its twelve timed releases occurred 2.037 to 2.061 seconds after the last action, with no page errors.

List-state-stability initially failed because it asserted the superseded unlimited focus hold. Its frame and geometry assertions remain unchanged; the fixture now presses the actual row, verifies that the list received pointerdown, and keeps the pointer pressed during its measurements. The corrected suite passes in 79 seconds across both themes, three widths and two text scales. The first attempted fixture correction pressed a list-boundary point that missed the list at wide sizes; that failed receipt is retained too.

Private evidence folders are `%TEMP%/dinah-chat-action-hold-first`, `%TEMP%/dinah-chat-action-hold-state-corrected` and `%TEMP%/dinah-chat-action-hold-state-row-press`. The initial folder includes the new regression's frame traces and screenshots. Only the focused browser suites ran; no full browser run, installation or native installed-window acceptance was performed.

## Handover

The Integrator should receive this branch under `docs/guides/merge-and-install.md`, rerun the six named suites on the merged tree, and perform the guide's remaining integration steps. The handover message supplies the final staged forbidden-terms guard and branch termscan receipts. The local e2e dependency junction is removed before handover. Front End retains corrections until the Integrator accepts the branch.

## Receiving correction

The integration run found that chat-recent-order still asserted an unlimited focus hold. This correction updates that assertion for the approved behavior change, preserving the activity-order, filtering and stable-tie checks. The suite now observes the reordered row while focus remains on the same chat, without blurring it. No product code changed.

All seven suites pass in the correction run: chat-recent-order, list-action-hold, list-pointer-hold, list-state-stability, list-row-layout, chats-busy-count and chat-states. The hidden, headless run took 147 seconds with two suites at a time. Its complete logs and screenshots are in `%TEMP%/dinah-chat-action-hold-receiving-correction`. The earlier integration failure remains retained by the Integrator. The Integrator should rerun these seven suites when receiving the correction child.
