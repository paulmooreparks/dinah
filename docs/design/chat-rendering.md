# Streaming chat rendering

## Provider continuation ownership, 25 October 2026

Provider notifications can arrive before the continuation or recovery RPC returns. An early notification can create a temporary chat controller for the destination session while the original conversation controller still owns its window. When the original controller adopts that session, it must transfer pending rendering from the temporary controller, remove its registry entries and dispose its subscriptions before rekeying the window. Otherwise both controllers consume subsequent events and append every streamed chunk and turn-completion row twice.

The adoption keeps early destination rows after the prior conversation and provider boundary. It preserves their pending rendering and message state. The provider-continuation regression delivers a notification before the RPC reply, then streams several chunks and completes a turn. It requires the early text, subsequent message and completion row to appear once. The regression fails against the preceding revision and passes with controller retirement. This reproduces the reported corruption without requiring a live provider; confirmation against a fresh Claude continuation remains a native application check.

## Freeze investigation, 24 October 2026

Paul reported that Dinah Desktop intermittently stopped accepting input and updating its display while Codex chats continued running. The application log contained tool failures but no corresponding crash; the crash log was empty. A process sample taken during a responsive interval showed the renderer using more CPU than the Go window process or Codex. That sample does not establish what those processes did during a freeze.

The running window started at 10:19 and used the build from `7fa62e7` with uncommitted changes. The asynchronous chat delivery build was installed at 10:52 and had not been loaded by that window, so its new delivery code could not have caused this instance of the symptom. One saved Codex conversation contained 11,756 tool update records. Those records are fragments and state changes, not separate commands.

## Reproduction and cause

A Chromium CPU profile reproduced the stall in Dinah's chat renderer using synthetic command output, without calling a model. The Codex adapter forwards each `item/commandExecution/outputDelta` as a tool update. For every update, Dinah rewrote the visible output, rebuilt the folded group's summary and called `scroll()`, which reads `scrollHeight`. These alternating writes and geometry reads forced layout repeatedly. Summary updates and scrolling continued even after the output reached its 32 KiB display limit.

The profile placed most samples in `scroll()`. The regression fixture measured 5,002 layouts for a burst of 5,000 updates. The rendering path was present in the initial `ae3bb88` source snapshot, before the recent provider handoff and asynchronous messaging changes. Codex's small output deltas amplify a shared host defect; the evidence does not establish an upstream Codex fault or rule out additional causes of native-window stalls.

The same isolated fixture produced these measurements. Each case sent one command's updates in a burst, with ANSI color sequences and text. These are synthetic burst measurements, not live-provider response times or whole-application latency guarantees.

| Updates | Before, synchronous handling | After, synchronous handling |
| --- | ---: | ---: |
| 100 | 100.2 ms | 0.3 ms |
| 1,000 | 1,164.2 ms | 1.4 ms |
| 5,000 | 3,195.0 ms | 3.4 ms |

The before run also recorded a 3,879 ms browser long task, including the resulting observer work. CPU profiles and measurement JSON are kept locally under `%TEMP%/dinah-freeze-20261024/` as `before.cpuprofile`, `after.cpuprofile`, `before.json` and `after.json`.

## Rendering contract

Tool updates change their bounded in-memory output immediately. Dinah schedules that output and affected group summaries through its shared frame queue. A text-only delta does not require recounting the tool group. Completed history replay enters the same queue. Status changes, ANSI colors, escaped text, output limits and background-chat ownership retain their behavior. PUDL's distributed files are unchanged.

Paul subsequently observed a freeze while typing, with commands executing in this chat and in a chat in another workspace. The native window was still the process started at 10:19, so this was another observation on the older build. Both chats share the renderer's main thread, which allows output rendering in either chat to delay keyboard input.

The `chat-streaming` browser suite checks a layout budget for 5,000 updates, final content and status, background ownership, composer input and replay. It also streams output into two chats while sending individual keystrokes to the front chat, verifies that input events arrive during streaming, and checks the complete draft and focus afterward. Both streams must finish in their own windows. This concurrent scenario passed with the rendering fix. The layout budget is expressed in layout count rather than elapsed time so machine load does not determine correctness. Native-window behavior under Paul's real workload still needs confirmation after restart.

## Shared frame scheduling, 24 October 2026

Paul approved a shared, event-driven rendering scheduler after the investigation. Each queued view has a stable identity, a visibility root, a cooperative draw callback and an optional callback for scrolling after frame writes. Repeated invalidations coalesce into the latest state. The scheduler requests one animation frame for the application and stops requesting frames when it has no visible work.

The scheduler admits rendering work for 6 ms per frame. Unfinished jobs move to the tail of the queue, so another view gets its turn before the busy view continues. Chat messages, tool rows and tool group summaries yield between entries. Bus conversations yield between messages. Geometry callbacks follow the admitted writes. This is a cooperative budget: one Markdown render, a browser layout or a library callback cannot be interrupted halfway through and may exceed it. The budget does not promise a maximum input latency.

Minimized windows, hidden dock tabs and closed chat views retain dirty state without painting. Window changes, root size changes and document visibility changes wake pending work. A visible window behind another remains eligible because it can still be partly visible. Closing a retained chat does not stop its agent; disposing a deleted view cancels its queued rendering and disconnects its observer. An early session's pending work transfers to the chat that claims it. A saved conversation remains visible until the reconnect's replacement log finishes rendering, even if that work spans several frames.

The shared queue serves chat messages, tool output, plans and streamed usage, the Chats and Workspace chat lists, automatic Workspace tree refreshes, In Progress, Bus overview and conversations, room membership and shell terminal titles. User actions keep their immediate handlers, with affected list views updated on the next admitted frame. Terminal bytes still reach xterm in order, because its parser owns terminal state and its renderer already batches screen painting. Agent execution, permission decisions, transcript persistence and asynchronous message delivery do not depend on frame scheduling.

The `render-scheduler` suite uses a controlled clock to check coalescing, budget exhaustion, fairness, idle behavior, visibility, cancellation, reentrant invalidation and error isolation. The browser streaming suite additionally checks that a minimized chat retains output and its final status until it is restored.

## Drawing only the block still arriving, 27 October 2026

Dinah drew a streaming message again from its first character on every admitted frame, so the work for each chunk grew with the message and a selection in it was lost. Dinah now draws only the part still arriving. The chat's Markdown renderer reads a line at a time, and a blank line it reaches outside a code block or table ends every paragraph and list. The text before the last such line therefore draws the same whatever follows it. A message keeps the nodes drawn for that text and draws again only the text after it.

Dinah finds the settled point by consuming lines as the renderer does, so a blank line inside a fence or a table never counts, and it reads only complete lines, so an open fence settles nothing after its start. The kept nodes are discarded and the message drawn again from the start when its text no longer begins with what they show, when they were removed, or when the paged view of an oversized message replaced them. Diagrams already drawn in kept blocks are no longer replaced and redrawn from the cache on each chunk.

The `chat-stream-markdown` browser suite streams a message five characters at a time and compares it after every chunk with a full redraw of the text so far. The cuts fall inside fence markers, table rows and an unclosed fence. It checks that a settled paragraph and code block are the same nodes at the end, that the streamed message equals the same text arriving in one chunk, and that 150 streamed paragraphs remove at most two nodes per chunk. Against the preceding revision, the kept-node and removal checks fail; the earlier code removed 11,325 nodes for those 150 chunks.

On 28 October 2026 markdown-it replaced the hand-written renderer, and the settled point now comes from markdown-it's own parse of the text's complete lines rather than from blank lines. docs/design/chat-markdown.md describes how it is found.

## Large history mistaken for a runaway chat

After restarting with the scheduler build on 24 October 2026, Paul reported slow startup and a chat whose title began with `<command-name>/clear</command-name>`. Messages kept appearing until he closed the chat, after which performance improved.

The matching workspace pin refers to Codex session `01a1568e-eca4-7302-bed0-6517ddd307bb` in another workspace. Its first recorded user message contains the command markup. The startup chat log gives it a different title. Pin titles are stored independently, so an old provider preview can remain in a pin after the conversation has a better name. Opening a pin uses its session ID; the title is not executed as a command.

The provider rollout contains 32,783,236 bytes and 22,221 response message records, comprising 321 user messages and 21,901 assistant messages. Its two records dated 24 October are thread settings applied during reconnection, at 11:16:38 and 11:29:13 Singapore time. There is no new task-start record during those reconnects. The apparent ongoing output was historical material replayed into the view. The transcript cache contains 623 merged messages, 12,448,328 text characters and 13,264,310 bytes of JSON. Its largest merged message has 494,321 characters. The local chat log records opening and reopening without new prompts or tool executions.

Startup first rebuilds the saved transcript's view, then reconnects the provider and rebuilds its full replay. The Codex adapter emits every item returned by `thread/resume`. The page bounds its history by row count, which does not bound the size of a row or a folded tool group. The frame scheduler yields between entries; it cannot interrupt the Markdown render of one enormous message. Closing the view defers its queued painting, which is consistent with Paul's observed improvement. No native CPU profile was captured during this occurrence, so the relative cost of transcript processing, Markdown rendering and layout remains unmeasured.

The remaining correction should bound initial history by both entries and text volume, with an explicit way to read earlier content from Dinah's complete local transcript. It must keep the provider connected for asynchronous delivery and preserve the full history for continuations. Provider control markup also needs exclusion from automatically derived titles, with existing pin labels reconciled against the current session title. These corrections are proposed here, not implemented by the scheduler change. The installed build was left unchanged while Paul tested it.

## The composer toolbar's folding, 28 October 2026

Paul asked for glyphs in the chat's toolbar so that it would fold more gracefully, without everything collapsing quickly under Agent. PUDL folds a toolbar in stages: every group first shows its controls as glyphs alone, and only then do groups fold, lowest priority first, into a menu button with the group's glyph and name, then into that glyph alone, and at last into the toolbar's More menu. A control with no glyph keeps its words at the glyph stage. The provider, the mode and the model had none, so the Agent group stayed at full width until it folded whole.

Each of them now has a glyph from PUDL's set: `app` for the provider, the program that answers the chat; `shield` for the mode, which decides what the agent may do without asking; and `tag` for the model, whose name it stands for. PUDL has no glyph made for any of the three. The other controls already had theirs: `clip` for Add, `slash` for Skills, `branch` for Subagents, `close` for Drop the Queued Messages and `square` for Stop. Send, Queue and Steer sit beside the prompt, outside the toolbar. As glyphs alone, the three keep the one in effect in their accessible name ("Model: Claude Opus 5.5 Medium") and in their tooltip, which is the command's label with the one in effect on a second line. The toolbar shows no group names before its groups, since they cost room the glyphs need; a folded group's button still says its name. The priorities are unchanged: Message folds first, then Agent, and Turn, at the row's end, last.

In a chat that is working with a subagent, the stages by window width are these. At 1,200 pixels every control shows its glyph and words. At 900 the Agent and Message groups show glyphs alone and Turn keeps Subagents and Stop in words. At 700 every group shows glyphs alone. At 520 each group is its menu button with glyph and name. At 380 Agent and Message are in More and Turn is its glyph alone. The `composer-fold` browser suite narrows the window in steps of 20 pixels and checks that no group folds into a menu while another still shows its words, that the Agent group shows glyphs alone before it folds, that Agent is never folded further than Message, and that every control in sight keeps its accessible name.
