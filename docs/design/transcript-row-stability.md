# Transcript row stability

The transcript must retain a measured row when its contents change. An element's last remembered size belongs to that element, as specified in [CSS Sizing Level 4](https://www.w3.org/TR/css-sizing-4/#last-remembered). Replacing the element discards that size. New log elements and groups moved from the live-work area also need an initial size before automatic content visibility can skip their contents.

The audit runs against the real frontend with invented messages and a stubbed desktop bridge. It records the state immediately after a change and in every subsequent animation frame. The immediate sample documents synchronous work before a paint. Assertions about the position of later text use the subsequent frames, since expanding a disclosure changes its height synchronously before the transcript follows its end. Height assertions include the immediate sample. A frame must not overshoot the range between the old height and the intended final height.

The regression must first fail on the unchanged source. A fixture must begin with a nonzero measured row, and a change to a retained row must preserve that element. Dismissal, deliberate history replacement and moving a completed work group have separate identity expectations. These operations may change the intended height, but they must not introduce an intermediate estimate.

## Source inventory

The inventory includes the row-producing and row-mutating code in `chat-applet.js`, its lifecycle helper and the shared text, failure-summary and transcript navigation modules. It excludes menus, settings and the chat browser's list of chats, since those elements are outside the transcript.

| Site | Operation | Check |
| --- | --- | --- |
| `arrivingCard`, `deliveredCard`, `teamSentCard` | A waiting card becomes a delivered card; the accepted preceding change retains the row. | The existing answer stability guard and the ordinary delivered-message checks cover this path. |
| `arrivingMore` | A count update removes and appends a replacement row. | The frame audit checks retained identity and later text. |
| `drawTool`, `toolRow`, `summarize`, `workHeading` | Tool output is rebuilt; titles, states and group summaries change. | The frame audit changes an expanded tool's output and status. |
| `appendChunk`, `draw`, `drawMarkdown` | Thought text and Markdown blocks are updated or rebuilt inside their rows. | The frame audit checks a visible thought, a replacement snapshot and a settled-block append. |
| `drawMessageImages` | A message's image gallery is rebuilt inside its retained row. | The frame audit redraws the same decoded invented image. |
| `drawPlan` | The plan's list is rebuilt inside its retained row. | The frame audit changes a plan's status. |
| `drawSubagentRow` | Text changes and the state badge is replaced. | The frame audit changes a subagent's state. |
| The `agent:steer` handler and `recoveryButton` | The message label changes and a recovery button may be added. | The frame audit checks the actual submitted steering row. |
| `recoverSteering` | Recovery creates a row, and Dismiss removes it. | The frame audit dismisses an invented recovered message. |
| `drawQueue` and the `agent:sent` handler | The composer count changes; sending adds a new transcript message. | The frame audit changes the queue count. |
| `signInNote` | A notice above the activity area is hidden or updated. | The frame audit dismisses a sign-in notice. |
| PUDL disclosure controls and `bounded-text.js` | Delivered messages and work groups fold; oversized text enters, pages within and leaves the bounded view. | The frame audit folds a delivered message and changes the bounded view. |
| `group`, `showLiveWork`, `finishWork` | A group moves between the live-work area and the transcript. | The frame audit records completion while later text is present. |
| `dinahFailures.sync`, `dismiss` | The summary count changes or its row is removed. | The frame audit changes the count and dismisses the summary. |
| `trimLog`, `trimMarker` | Earlier rows are removed and the retained marker's text changes. | The frame audit checks the marker and crosses the 600-row trimming boundary. |
| `transcript.updateSeen` | The unread divider is moved or removed while the current reader anchor is retained. | The transcript navigation suites cover this path; the frame audit checks an update while reading above the end. |
| `drawSaved`, `drawSavedCopy`, `presentHistory`, `session-lifecycle.js` | A detached history is rebuilt and swapped for the displayed saved copy. | A second frame fixture completes a replay of identical history. |
| `applyContinuation` | Rows are moved to a continuation's log before the window is rekeyed. | The provider-continuation suite records every retained row through confirmation and handoff. |
| `clearArriving`, chat deletion and view disposal | Waiting cards or the whole view are intentionally removed. | The frame audit closes a chat with arriving cards. Explicit deletion and disposal remove the selected view, which leaves no retained transcript in that view to measure. |

## Changes

The transcript measures each newly attached activity row with ResizeObserver before enabling automatic content visibility for that row. Its content-box height becomes the intrinsic-size fallback and stays current when the row changes size. This also covers a live group that grows before it enters the transcript. Later browser measurements can still update the last remembered size. Hidden rows do not enable automatic content visibility until they have a nonzero height; removal and view disposal release observers. This uses the documented [Resize Observer content rectangle and notification model](https://www.w3.org/TR/resize-observer/). The change requires an initial layout of the retained rows, whose existing limit is 600. Subsequent distant layout can still be skipped.

The waiting remainder count updates its existing row in place, so it stays before messages that arrived later. Paging oversized text uses the chat's existing position-preserving render function. The page change and scroll correction finish in the same render operation. Trimming defers tail positioning until the new message's contents have been drawn.

Host-result messages use the existing typed `host_result` field, which is present in both arrival previews and delivered metadata. Their waiting and delivered frames are 12rem high, with a 4rem header and a scrolling body. This covers the restart's four carry-on messages without interpreting prose or changing a service contract. Decisions answer cards keep their existing 20rem frame.

A provider continuation retains the original row elements and heights. Its existing preparation stage hides the composer; the evidence records that zero box separately, while the guard checks retained transcript rows, the window and taskbar. This audit does not change provider-switch presentation.

The fixtures use invented content. They do not establish native-window acceptance. The review record gives the baseline failures and final verification.
