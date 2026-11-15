# Transcript row audit

The audit found six paths that need correction. The waiting-message count replaced its measured row and changed its position in the log. Completed work groups, replacement history and the retention boundary briefly used estimated heights. Paging oversized text corrected the following position one frame after drawing. All four carry-on messages changed height between their preview, delivery and rendered body.

The source inventory is in [the design note](../design/transcript-row-stability.md). The implementation measures rows before allowing automatic content visibility, keeps the waiting count's row, and uses the existing position-preserving render path for oversized text. Host results have the same fixed frame while waiting and after delivery, with a scrolling body. The existing typed message kind identifies them.

## Results by site

The table distinguishes intended changes to content height from a temporary fallback or a delayed scroll correction. A disclosure may expand once, and an arriving message may add its reserved space once. Those operations must not pass through another height or move a second time while their body is drawn.

| Site checked across frames | Baseline result | Final result |
| --- | --- | --- |
| Waiting remainder count | The old row was removed, later text changed position, and scroll position jumped by 100 pixels before returning. | Fixed; the frame guard passes. |
| Markdown snapshot replacement | The row stayed attached and had no intermediate height. | The measured transition remains stable. |
| Markdown settled-block append | The row grew directly to the intended height. | The measured transition remains stable. |
| Visible thought snapshot | The measured row remained in place. | The measured transition remains stable. |
| Tool output and status | The expanded tool retained its row and grew directly to its final height. | The measured transition remains stable. |
| Plan list rebuild | The measured row remained in place. | The measured transition remains stable. |
| Long delivered-message expansion and collapse | The disclosure changed height once, with no estimated-height frame. | The measured transition remains stable. |
| Oversized text entering and leaving its paged view | The row retained its identity and changed directly to its final height. | The measured transition remains stable. |
| Oversized-text page navigation | The following text moved by 8,448 pixels for one frame before the tail correction. | Fixed; the frame guard passes. |
| Sign-in notice dismissal | The viewport gained room once; the retained row did not collapse. | The measured transition remains stable. |
| Queue count and text | The measured transcript stayed stable. | The measured transition remains stable. |
| Subagent state badge replacement | The measured row retained its height. | The measured transition remains stable. |
| Sent-image gallery rebuild | The retained row did not collapse in the invented decoded-image fixture. | The measured transition remains stable. |
| Trim marker count | The measured marker retained its height. | The measured transition remains stable. |
| Work group expansion and collapse | The disclosure changed directly to its intended height. | The measured transition remains stable. |
| Recovered-message dismissal | The selected obsolete row was removed without an intermediate height in retained rows. | The measured transition remains stable. |
| Failure summary count and dismissal | The summary changed to its intended size or was removed without a transient estimate. | The measured transition remains stable. |
| Steering from pending to delivered | The same measured message row remained attached. | The measured transition remains stable. |
| Live work completion | The group changed from 56.594 to 133 pixels for a frame, then returned to 56.594 pixels. | Fixed; the frame guard passes. |
| Queued message added to the transcript | The existing measured row retained its height. | The measured transition remains stable. |
| Retention at 601 rows | The later text overshot its final position by 12 pixels for one frame. | Fixed; the frame guard passes. |
| Unread divider while reading earlier messages | The reader's measured row remained in place. | The measured transition remains stable. |
| Closing a chat with arriving cards | The obsolete cards were removed without an intermediate height in retained rows. | The measured transition remains stable. |
| Replay replacing the displayed saved copy | The answer moved past its intended position for one frame while the replacement log used estimates. | Fixed; the frame guard passes. |
| Provider continuation | Every original row stayed attached with its measured height through the handoff. | The measured transition remains stable. |
| Four carry-on message bodies | Every body failed the equal-height delivery check on the unchanged source. | Fixed; the frame guard passes. |

Provider switching already hides its composer during preparation. The continuation record includes the resulting zero rectangle. The retained-row guard checks the transcript, window and taskbar; this change does not alter the switch's presentation.

New file, status, turn-end and provider-switch rows are constructed before their first measurement. Their construction now uses the same measured-row boundary. Explicit deletion and disposal remove the chosen view, leaving no retained transcript in that view to compare.

## Verification record

The unchanged baseline is commit `a9538c3c` in a detached worktree. The final frame fixture ran there without product changes and failed the waiting count, oversized-text paging, work-group completion, retention and replay checks. The carry-on fixture separately failed its delivery-height check for every one of the four real message bodies. [The compact baseline record](transcript-rows-before-2026-11-15.json) retains every sampled height, later-text position and scroll position for the row audit.

The baseline runs are in `%TEMP%/transcript-row-baseline-final-20261115` and `%TEMP%/carry-on-baseline-final-20261115`. The complete records include geometry and invented text. The summary does not depend on screenshots to establish intermediate frames.

The first diagnostic run established the count and work-group defects. A later fixture waited on a lazy image outside the viewport and was stopped after verifying its own process tree. The corrected fixture explicitly requests that image and bounds decoding. An early replay fixture also returned a recovered message for every chat; it now returns that message only for its intended chat. Neither fixture correction changes a product assertion. A provisional replay lifecycle edit was removed after the evidence identified the replacement log's missing sizes.

The first combined regression run passed fifteen related suites and failed the work-group frame check. The first shared measurement was too early for a live heading that grew later. The observer now keeps each fallback current until removal, rather than stopping after the first positive size.

The final focused run passed carry-on-stability, decisions-answer-stability and transcript-row-stability together in 28 seconds through FIFO ticket 2056. The row suite measures 27 mutations plus replacement history; the carry-on suite measures all four bodies in fresh fixtures. [The final row record](transcript-rows-after-2026-11-15.json) and [the carry-on comparison](carry-on-frames-2026-11-15.json) preserve the before/after values. The final full records and screenshots are in `%TEMP%/transcript-row-after7-20261115`.

The fixture bytes are identical in the baseline and changed-source worktrees. Their SHA-256 values are `1230d355424250bad5a7668cc50f2e603c5c681c50d99817bc4bb1a884874881` for transcript-row-stability.js and `69e3383574029ae93541aae25aad5713f9386066bfd6dabfa55e9ad7d4eb3f3f` for carry-on-stability.js.

The focused frontend ownership, stylesheet ownership, asset-exclusion and UTF-8 Go checks passed in 0.478 seconds through FIFO ticket 2055. Whitespace and JavaScript syntax checks passed.

The final combined browser run passed all sixteen related suites in 114 seconds through FIFO ticket 2057, on unchanged source. They were app-ui4, carry-on-stability, chat-arriving, chat-replay-isolation, chat-saved-restore, chat-scroll-place, chat-stream-markdown, chat-stream-tail, chat-tool-retention, decisions-answer-card, decisions-answer-layout, decisions-answer-stability, delivered-messages, provider-continuation, transcript-retention and transcript-row-stability. The existing 3,000-part history assertion still passed its three-second limit. Logs, frame records and screenshots are in `%TEMP%/transcript-row-regression-after7-20261115`.

The interrupted carry-on screenshot was inspected. [A committed copy](transcript-carry-interrupted-2026-11-15.png) shows the fixed frame in the narrow floating chat. Geometry records establish intermediate-frame behavior. The detached baseline worktree was left clean after its copied fixture files were removed.

## Limits and ownership

These are headless Edge checks of the real frontend with invented bridge data. They do not establish native-window acceptance or repeat Quality's backend restart acceptance. The size observer requires an initial layout of up to the existing 600 retained rows; distant layout can still be skipped afterward. The large-history and streaming checks cover the relevant existing behavior.

The work stays on `frontend/transcript-row-stability`. No push, merge, installation, visible process or real-home write is part of this task.

The requested next action is for Dinah Team Lead to review the branch and integrate it with the other shared fixes. Front End keeps ownership until that review accepts it.
