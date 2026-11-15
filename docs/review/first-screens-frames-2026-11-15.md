# First-restart screen movement

Quality checked the page at master `6c8efb7a265214616713f2489ad665e470227178` on branch `quality/first-screens`. No product file changed. The independent fixtures use the same headless Edge page host and animation-frame sampling method as `decisions-answer-stability`.

## Measurement boundary

Each transition begins after two setup frames. The recorder takes the bounds of all CSS-visible page elements intersecting the viewport after overflow clipping, records every animation frame during the action and asynchronous response, then records eight more frames after the expected state is observed. A change of at least one CSS pixel in x, y, width or height counts as movement, matching the existing frame test's tolerance. The JSON keeps every sampled rectangle and identifies added and removed elements separately. A newly opened dialog or removed card is not classified as a moved retained element.

The recorded frame numbers are relative to the action, starting at one. `afterTransition` in each JSON marks when the fixture observed the target state; the following eight frames are always included. A state that takes a service poll to arrive therefore has more than eight total frames. The screenshots show the settled result, not eight separate screen captures. The frame evidence is the per-frame geometry.

The page and its rendering code are real. Service replies, elapsed two-minute state, update downloads, file removal and bus delivery are invented. This verifies layout transitions and does not repeat backend timing, installation, removal or restart acceptance. The Settings viewport is 1280 by 900; the chat and Decisions viewport is 1440 by 1000. These are floating-window checks in the default light theme. They do not claim all viewport sizes, themes or native WebView behavior. Bounds do not establish pixel occlusion by an overlapping window. The final recorder covers the whole page, including window chrome, menus and neighboring controls; the first exploratory captures covered narrower roots.

## Fixture corrections retained

The first restart fixture matched both the full overdue card and the notification menu entry. It was corrected to use the card under `#questions`. The second attempted an unavailable `pudlWindows.reveal` helper and was corrected to use the existing workbench open action. The third assigned duplicate numeric message IDs to the generated requests, which the page correctly refused; the IDs now start at one.

The initial Settings capture kept the inventory at the top of a scrolling window, leaving the release and removal controls outside the viewport. The final Settings run scrolls those controls into view before establishing the baseline. The initial evidence is retained but does not support claims about the off-screen controls.

The initial carry-on probe used short invented wording. The dedicated carry suite uses all four bodies emitted by `resumeText` in `restart_pause.go`, with the Dinah sender and host-result kind used by `Bus.EnqueueResult`. Each body begins in a fresh chat fixture.

## Evidence

All browser runs use the shared FIFO through `run.ps1`, with one suite at a time. Existing dependencies are resolved through NODE_PATH; nothing was installed. Each transition has a JSON record and a settled PNG. The runner retains full logs, including the fixture failures above.

Final transition results and artifact locations follow below.

## Findings for routing

Three areas moved on this pinned build.

1. Starting the pause changes the status label from Update to Restarting. In frame 2 the status control grows from 133.016 to 152.344 pixels, moves left by 19.328 pixels, and shifts Arrange taskbar and the adjacent navigation bounds. Cancelling reverses this in frame 6. The status change has already been observed when the asking movement occurs.
2. The Updates progress element and its neighboring text share a changing width. Downloading moves the text left by 3.609 pixels in frame 1. The first known-size progress reply moves it right by 75.063 pixels in frame 109. Installation completion moves it left by 156 pixels in frame 12 and shrinks the progress element from 156 to 2 pixels. These are changes to the element bounds, beyond the intended change to the filled portion of a progress bar. The surrounding window and buttons remain fixed.
3. Every actual carry-on body changes chat geometry in frames 2 and 3. For cancelled, paused and ended messages, arrival grows the transcript in frame 2 and moves earlier text up by 62 pixels in frame 3. Delivery moves that earlier text down by 62 pixels in frame 2, then up by 19 pixels in frame 3 as the body appears. The longer interrupted message moves it up by 110 pixels on arrival, down by 110 pixels on delivery, then up by 67 pixels in the next frame. The new delivered body grows by 72 pixels for the first three variants and 120 pixels for interrupted. These observations include scroll-induced movement; the audit does not assign a code cause or decide which movement the product should permit.

No retained visible element moved in the measured Set-aside confirmation, removal-in-progress or removal-result transitions. No retained visible element moved for the Decisions count transitions 0 to 1, 1 to 9, 9 to 10, 10 to 99, 99 to 103 and 103 to 0. The overdue card, Restart Now confirmation and declining that confirmation add or remove content without moving retained elements in this fixture.

## Transition inventory

The frame numbers below start with the action. None means that no recorded rectangle moved by one pixel. The committed JSON summary retains each changed rectangle and the SHA-256 of each full temporary frame record. The two short carry-on probes are exploratory; the four named variants provide the actual-message result.

| Transition record | Result | Frames with movement |
| --- | --- | --- |
| aside-confirm | Stable | None |
| aside-removed | Stable | None |
| aside-removing | Stable | None |
| update-available-checking | Stable | None |
| update-available-reply | Stable | None |
| update-current-checking | Stable | None |
| update-current-reply | Stable | None |
| update-downloading | Moved | 1 |
| update-error | Stable | None |
| update-install-request | Stable | None |
| update-installed | Moved | 12 |
| update-progress | Moved | 109 |
| carry-cancelled-arriving | Moved | 2, 3 |
| carry-cancelled-delivered | Moved | 2, 3 |
| carry-ended-arriving | Moved | 2, 3 |
| carry-ended-delivered | Moved | 2, 3 |
| carry-interrupted-arriving | Moved | 2, 3 |
| carry-interrupted-delivered | Moved | 2, 3 |
| carry-paused-arriving | Moved | 2, 3 |
| carry-paused-delivered | Moved | 2, 3 |
| carry-on-arriving | Moved | 2, 3 |
| carry-on-delivered | Moved | 2, 3 |
| decision-count-0 | Stable | None |
| decision-count-1 | Stable | None |
| decision-count-10 | Stable | None |
| decision-count-103 | Stable | None |
| decision-count-9 | Stable | None |
| decision-count-99 | Stable | None |
| restart-asking | Moved | 2 |
| restart-cancel | Moved | 6 |
| restart-now-decline | Stable | None |
| restart-now-question | Stable | None |
| restart-overdue | Stable | None |

The final Settings run passed its capture assertions through FIFO ticket 2010 in 14 seconds, the actual carry-on run through ticket 2011 in 12 seconds, and the restart/count run through ticket 2012 in 10 seconds. Passing capture assertions means the eight frames were recorded; it does not classify a moving transition as stable. No page errors were reported.

The full records, screenshots and runner logs are in `%TEMP%/quality-first-screens-settings-2`, `%TEMP%/quality-first-screens-carry` and `%TEMP%/quality-first-screens-restart-final`. The settled removal and interrupted-message screenshots were inspected. Syntax and whitespace checks passed. Only tests and this evidence record were added, with no product fixes, push, merge or installation. The requested next action is for the lead to route the three movement findings; this report does not claim that later master changes still exhibit them.

