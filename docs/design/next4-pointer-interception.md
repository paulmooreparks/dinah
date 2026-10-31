# Reading a followed transcript before activating its links

Release stopped the full browser gate on merged df6429cff37f477acfa73d12464d89c322179f78. The next4 guide-chip click timed out while its own New Chat window title intercepted pointer input. The original full-gate failure remains preserved under the release output at %TEMP%/dinah-release-terminal-20261101-011859. A copy and all investigation evidence are retained in the Front End worktree at .diagnostics/next4-interception-20261101.

## Captured behavior

The first isolated, instrumented next4 run passed. It captured the chip above the transcript viewport before and after lib/windows.js revealed its owner. The chip and intercepting header belonged to the same active window. Raising the window therefore did not remove the obstruction. No other window covered the target.

A controlled reproduction used the ordinary locator scrollIntoViewIfNeeded operation followed by two animation frames. In each of three consecutive cycles, automatic scrolling put the chip at y404.047 with transcript scrollTop155 and the chip was the hit-test target. Following remained true. After the frames, the transcript returned to scrollTop346 and the chip moved to y213.047, above the viewport top y247.453. The hit-test target became the same window's H2.win-title. The record includes target/header owner IDs, ancestor geometry and scroll offsets, active-window state, screenshots and a Playwright trace.

A separate CDP debugger run paused transcript.check and inspected its local following value and log dimensions. Following remained true during the automatic navigation scroll, so the existing callback queued restoration of the tail. Debugging was disabled before browser cleanup. No production logging or instrumentation was added.

An upward wheel over the transcript set following=false and moved the chip to y346.453 at scrollTop166. The guide remained the hit-test target both before and after the existing window reveal. All ordinary guide, external-address and file-link clicks then passed their original destination assertions. This provides the user action omitted by the fixture.

The original release failure did not capture geometry. The controlled reproduction establishes the same owner/header interception and its callback ordering; it does not establish machine load as the trigger. The initial isolated pass is retained and does not dispose of the original full-gate failure by itself.

## Correction and compatibility

The correction changes next4 only. Before activating the links, the fixture moves the pointer over the transcript and sends an actual upward wheel. It waits for following to stop, crosses two animation frames, and asserts that the guide is inside the viewport and receives pointer input. The existing lib/windows reveal and ordinary clicks still execute. Destination assertions are unchanged. There is no forced click, direct scroll assignment, longer deadline or product window change.

The relevant files are byte-identical between released c8c6038 and merged df6429c: transcript.js, chat-applet.js, chat-text.js, next4.js, e2e/lib/windows.js and dinah.css. Their hashes are recorded in relevant-source-comparison.json. The skins and terminal merges did not change the follow/navigation contract. Existing incident and prior diagnostic reports remain separate.

## Verification

The controlled reproduction passed in 10 seconds and retained all three automatic-scroll/tail-reset cycles plus the successful real-wheel interaction. The corrected ordinary next4 suite passed in 8 seconds. Its pre-click screenshot was inspected and shows the guide within the visible transcript. The candidate handoff records the immutable successor and focused-test provenance. Quality retains independent review; Release must run merged-master gates before deployment. Front End performed no install, push or restart.
