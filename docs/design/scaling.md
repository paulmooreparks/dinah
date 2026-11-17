# How the page's cost grows with open chats and windows

Paul asked on 17 November 2026 why the application slows as more chats and applets are open, by more than their number would explain, and for tests that find where. This document records the benchmark, what it found, what was changed, and what is left.

## The benchmark

`cmd/dinah-desktop/frontend/e2e/bench/scaling.js` opens 1, 8, 16 and 32 chats in the real page, each in its own window with a history of 120 messages, beside 100 closed chats in the lists and a saved UI state of about 600 KB. It then performs a fixed set of ordinary actions and reads the browser's own counters for each one (main-thread task time, script, style and layout, through `Performance.getMetrics`). Each action touches one chat, so an action whose cost rises with the count is doing work for chats it does not concern.

```
cd cmd/dinah-desktop/frontend/e2e
node bench/scaling.js <outFolder> 1,8,16,32
node bench/scaling-table.js <outFolder>
```

These environment variables go further.

- `PROFILE=action,...` prints a CPU profile of those actions at the largest count: self time by function, the browser's own functions charged to the page function that called them, and time inside the vendored window library by the call that entered it.
- `TRACE=action,...` sums the browser's timeline events for an action. With `REASONS=n` it also says why style was recalculated and how many elements each recalculation restyled.
- `LAYOUT=stack` lays the windows over one another at sixteen places, as a long-used desktop has them. The default is the layer's own cascade.
- `PUDL_DIR=folder` serves another copy of the window library, so a candidate change to it can be measured without touching the vendored files.
- `VARIANTS`, `EXPERIMENT`, `CUSTOM` and `PROBE` try a stylesheet, a script or a module in the same page. The probes kept in `bench/probes/` are the ones this investigation used.

The benchmark drives headless Edge with the bridge stubbed, so it measures the page and nothing of the Go host, the providers or the native window's composition. Its numbers move by a third with what else the machine is doing, so two runs are compared only when they were taken together.

## What it showed

The table gives main-thread milliseconds for one unit of each action before the changes of 17 November and after them. The machine was busier during the second run: the one-chat column, which the changes do not touch, is about 30% slower there.

| Action | 1 chat | 8 chats | 16 chats | 32 chats |
| --- | --- | --- | --- | --- |
| A keystroke in the front chat | 1.3 → 1.7 | 3.0 → 3.3 | 12.4 → 7.3 | 34.9 → 18.5 |
| A scroll step in the front transcript | 6.5 → 6.6 | 9.6 → 10.7 | 24.5 → 14.6 | 51.6 → 21.4 |
| A streamed chunk, front chat | 13.2 → 17.0 | 17.9 → 21.5 | 44.6 → 23.2 | 42.0 → 23.6 |
| A streamed chunk, a chat behind | 12.5 → 16.4 | 18.0 → 17.8 | 50.3 → 13.3 | 41.9 → 16.9 |
| A turn's end | 12.9 → 18.3 | 24.0 → 16.9 | 49.7 → 14.1 | 58.9 → 17.4 |
| A change of one chat's state | 29.8 → 40.8 | 61.4 → 63.2 | 66.7 → 55.2 | 96.6 → 76.2 |
| A window brought to the front | 16.0 → 16.5 | 74.1 → 72.4 | 99.9 → 80.6 | 230.9 → 156.9 |
| Opening every chat, in seconds | 2.0 → 2.0 | 4.5 → 4.5 | 10.3 → 9.0 | 27.1 → 21.7 |

A sample of a running installation with some forty chats agreed about where the time goes. Over ten seconds the page's process used 41% of a processor core and the Go host 4.5%.

## Causes found, and what was changed

**Every window was drawn, whatever lay on top of it.** A frame in which anything on the page changed cost 1.8 ms with one chat open and 12.4 ms with 32, most of it the browser working through the content of windows that could not be seen. The cost grew faster than the count because overlapping windows multiply the layers the browser must sort. A window whose content area is wholly covered by the windows above it now keeps its content mounted and does not draw it (`covered-windows.js`, tested by `e2e/covered-windows.js`). The rule is exact and errs toward drawing: rounded corners and a skin that leaves the frame clear are accounted for, and a window is drawn again in the same frame in which a drag, a key or a raise uncovers any of it. On a real saved layout of sixteen floating windows, three were wholly covered and five more showed two pixels or fewer.

**A streamed chunk restyled the chat lists.** Each chunk wrote a `hidden` attribute eight times with the value it already had, and each write made the browser restyle about 2,800 elements, the rows of every chat list among them. The transcript now writes such marks only when they change. The rest of that restyle comes from three stylesheet rules that use `:has()`. One was the application's own and is replaced by a class the script sets (`.row-with-actions`). The other two are in the window library's stylesheet and are listed below.

**Every transcript answered every change to any window.** Raising one window made each open chat lay itself out. A transcript now answers only when its own window came to the front, and it learns which window is in front from the layer's announcement, where it used to ask for a copy of the whole arrangement on every chunk.

**The whole saved UI state was sent for every change.** A long-used installation's state is some 600 KB in 600 values. Each save copied all of it, sent it across the bridge and compared it twice more, on the page's thread, a moment after any window moved or any transcript scrolled. The page now keeps each value as text and sends the changed values alone (`SaveUIStateChanges`). Each window's remembered place is a value of its own, where all of them were one 80 KB value that was rewritten whole.

**A change to one chat searched every open window.** With 32 chats the page holds some 300,000 elements. Looking up a window by its key, finding the controls that run a command, and refreshing the notification counts each asked the whole document, several times for every change of a chat's state. These now read the window layer's own children or a kept list (`helpers.js`: `windowOf`, `windows`, `windowState`; `commands.js`: `controlsFor`).

## What is left

Most of what remains is inside the vendored window library, which belongs to PUDL. The figures are from the profile at 32 chats.

- **Layout forced for each window on every redraw.** `restricted()` reads the layer's width once for each window while the redraw is still writing styles, which lays the page out each time. It is the largest single item in raising a window (about 32 ms of a raise) and in opening a chat (about 440 ms of an open).
- **One observer redraws everything for any window's state.** A change to `data-win-state`, `data-win-attention` or a dozen other attributes of any window redraws every window's title state, searches the document for every taskbar tab, and redraws the docks and the taskbar. It costs about 26 ms for each change of any chat's state.
- **Two stylesheet rules that use `:has()`.** `.topbar:has([data-menubar]) .status-item > .badge:is(...)` and the row actions rule that begins `:is([data-row-hot], ..., :has(> [aria-selected="true"], ...)) > [data-row-actions]`. With them, text inserted into any window restyles every badge and row on the page. A candidate without them halves the cost of a streamed chunk at one chat (13 ms to 6 ms).
- **A search of the whole document on every key.** `pudl-commands.js` line 642 costs about 2 ms a keystroke at 32 chats, and its `draw` and `refreshOtherRows` search the document again on every change of the commands.
- **Work that grows with the square of the window count.** `parentOf` and `stackOf` read an attribute of every window for every window, and the address is rewritten in full for every raise.

A candidate with only the first and third of these, measured through `PUDL_DIR`, brought a streamed chunk at 32 chats from 30 ms to 17 ms and a change of state from 106 ms to 76 ms in the same run.

Three things are left in the application.

- **Windows of which a strip shows are drawn in full.** A cascade leaves many windows showing a few pixels of content. Their transcripts give each row `content-visibility: auto`, which costs 5 ms a frame at 32 chats while saving nothing for a window whose size is not changing. Giving that to the front window's rows alone would cost a full layout of one transcript on every change of the front window, so it was measured and left.
- **The saved UI state still grows.** Answered decisions' drafts (340 KB of the 600 KB measured), scroll positions and window places of windows long closed are never removed. They no longer cost anything on a save, and they are still read at start.
- **The Go host has not been benchmarked.** The live sample says it is not where the time goes today.
