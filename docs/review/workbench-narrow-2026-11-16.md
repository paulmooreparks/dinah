# Narrow workbench delivery

Paul approved mockup d9f83271. This branch starts at master b265e8a7 and merges the approved mockup. Tested product source is a4c6657d. The branch contains no private product files.

The workspace layer now uses PUDL's documented data-win-narrow="maximized" policy with its default 640 CSS-pixel boundary. Workbench windows configured as fixed docks, including dock-only windows, receive data-win-narrow="keep". An ordinary window that the person docks still participates in the narrow policy and recovers its canonical placement on widening. No custom resize handler, placement calculation, stylesheet override or breakpoint was added.

## Verification

The new workbench-narrow browser suite passes 56 checks against the real workbench and mounted chat composer with invented content. Both themes cover 390, 1440 and 390 again, then the actual layer boundary at 641 and 640 pixels. The suite checks reachable title-bar controls, unchanged canonical placements and placement address parameters, restored rendered rectangles, dock exceptions, composer node identity and an unsent draft. It also opens a fresh window while narrow. Browser errors are zero.

All seven selected suites pass in 28 seconds. The other six cover chat placement, chat windows, passive startup, workspace toolbar, the chart and chart actions. The adjacent pictures show the actual product page with invented windows in light and dark at 1440 and 390 pixels.

A private snapshot of the reported feature-page fixture also passes against this product source, including added title-bar bounds checks in both themes at both widths. Its existing assertions were retained. Its scripts, product files, raw observations and hashes remain outside this public branch. No real provider or task execution was used.

The broader workbench suite fails its launcher-catalog expectation and then times out at its Skills setup on both the candidate and the unchanged b265e8a7 product baseline. It is not counted as a pass. Those matching receipts remain private; no assertion in that suite was changed.

The first new-suite run caught two fixture errors. It captured the chat's storage node instead of the mounted composer, and compared the whole address even though the launcher's existing responsive setting changes independently. The corrected checks retain the actual composer node and every window-placement address parameter. Raw whole addresses remain captured. The draft and canonical-placement assertions remain, and the rendered-rectangle check was added. Original failed observations remain private.

The complete profile package and exact-tip private-term, commit-message, branch-diff and local-path guards are run after this note is committed, with the receipt retained privately. Compiler output is excluded. These hidden browser checks do not establish installed native-window acceptance. No push, trunk merge or installation occurred. Lead owns integration and shipping.
