# Narrow workbench proposal

This clickable mockup addresses title-bar controls that leave the screen when a wider floating layout is viewed at a narrow width. It starts from shipped source b265e8a7 and uses the shipped PUDL, app.css and the approved appearance tokens. All window content is invented. No product code changes are included.

## Reproduction

The reported page runner reproduced the clipped run window at 390 pixels. Its content-width checks passed even though the title-bar controls were outside the reading surface. The original screenshot and runner output remain private.

A separate fixture reproduced the defect through the current b265e8a7 workbench using its real registration and opening paths with six invented ordinary windows. It opens them at 1440 pixels and then narrows to 390. The final Close button extends to 417.1875 pixels, beyond the viewport. The adjacent current-workbench picture records that result. Directly opening generic windows at 390 did not reproduce this case; those diagnostic observations are retained. The wider-to-narrower transition is required for this reproduction.

## Proposed behavior

The workspace layer opts into PUDL's documented data-win-narrow="maximized" policy. PUDL uses a default breakpoint of 640 CSS pixels, measured on the layer before subtracting docks. The proposal keeps that default. It introduces no independent breakpoint or placement algorithm.

At or below that width, ordinary windows fill the space left by unrestricted dock strips. A strip carries data-win-narrow="keep" and retains PUDL's dock behavior. The mockup's Quick Links strip stays available and follows the library's existing side-dock presentation. Content-sized windows retain the documented exception; this mockup covers ordinary windows and one dock strip.

The canonical window placement remains in PUDL's URL state. Widening restores the saved floating layout without remounting the windows or replacing it with the narrow presentation. Draft text remains in the mounted controls. Restricted placement controls use PUDL's own availability rules. The mockup reads effectivePlacement() for its explanatory status and never writes that derived value into saved state.

The governing API is documented in PUDL's [responsive workspace contract](https://github.com/paulmooreparks/pudl/blob/master/docs/RESPONSIVE-WORKSPACES.md), under Narrow placement. The viewport buttons, theme selector and policy comparison are mockup scaffolding.

## Walkthrough

1. Open index.html and switch among Chat, Notes and Run with the window tabs or the Open buttons.
2. Type in Chat, choose 1440 px, move a window, and return to 390 px. Widen again and check the placement and text.
3. Close Run with its title-bar button, then open it again. Try the same action from the keyboard.
4. Switch between light and dark. Turn off Keep windows in view to expose the original clipped cascade, then turn it back on.

## Verification

Both themes passed at actual browser widths of 1440 and 390, including the return to 390. Separate container-resize checks passed at those widths. The documented layer boundary was checked at 641 and 640 pixels. Close buttons remained within the viewport, and the document had no horizontal overflow.

The checks also retained draft text and canonical placement through resizing, closed and reopened Run, and exercised keyboard activation. The unprotected cascade comparison exposed clipped controls before the policy was restored. Browser errors were zero. The first mockup run caught an initialization callback before PUDL's DOMContentLoaded initialization; the corrected callback runs afterward. Original diagnostics remain private.

The adjacent pictures show the final mockup and the current-code reproduction. Raw geometry and runner output stay outside the repository. These are hidden browser checks against invented content. Installed native-window behavior is not claimed. Paul must approve the visible change before product implementation.
