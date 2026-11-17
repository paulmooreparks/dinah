# Brief for PUDL: what slows a page with many windows

This brief is for whoever works on PUDL's window layer and stylesheet. It lists five places in PUDL 0.107.1 where the cost of one small change grows with the number of open windows, with the measurement behind each and a way to check a fix. It was written on 17 November 2026 from an investigation in Dinah, which vendors PUDL unchanged. None of the changes it suggests alters a contract.

## Where the numbers come from

Dinah has a benchmark that opens many chat windows in its real page and times ordinary actions with the browser's own counters (`cmd/dinah-desktop/frontend/e2e/bench/scaling.js` in the Dinah repository; `docs/design/scaling.md` there describes it). Every figure below is from a CPU profile or a browser trace of that page in headless Edge with 32 chat windows open. The page then holds about 300,000 elements, and that is the number that matters here: a search of the whole document costs 2 to 5 ms at that size, and a forced layout costs more.

The benchmark can serve another copy of PUDL in place of the vendored one, so a candidate can be measured before it is released:

```
cd <dinah>/cmd/dinah-desktop/frontend/e2e
PUDL_DIR=<folder holding a changed dist> ONLY=session-state,raise-window,open-chat,stream-front,typing \
  PROFILE=raise-window,session-state node bench/scaling.js <outFolder> 32
```

`PROFILE` prints self time by function, the browser's own functions charged to the function that called them, and time inside PUDL by the call that entered it. Two runs are comparable only when taken one after the other, since the figures move by a third with the machine's load.

## The five findings

### 1. A layout is forced for each window on every redraw

`restricted(key)` in `pudl-windows.js` (line 1281) reads `layer.clientWidth`. The redraw loop calls it for every window (line 2612, `el.toggleAttribute('data-win-restricted', restricted(k))`) straight after `setPlacement` has written that window's styles, so every call makes the browser lay the page out again. `effectivePlacement`, `shownEdge`, `narrowLayer` and `menuCommands` reach the same read.

Measured: about 32 ms of each window raise and about 440 ms of opening one more window, as self time of `restricted`. It is the largest single item in both.

Suggested: read the layer's width once and keep it until the layer is resized (the layer already has a `ResizeObserver` and a `resize` listener for `refreshPolicy`), or read it once at the top of the redraw and pass it down.

### 2. One observer redraws everything when any window's state changes

The `MutationObserver` near line 6009 of `pudl-windows.js` watches the whole layer for `data-win-state`, `data-win-attention`, `data-win-running`, `data-win-progress` and a dozen more attributes. For a change to one window it runs `titleState` for every window, `document.querySelectorAll('[data-win-tab]')` with `windowMark` for every tab, `renderDocks`, `fitBarsSoon`, `renderDockGroups` and the taskbar's refresh.

Measured: about 26 ms for each change of any one window's state. An application whose windows are running agents changes those attributes many times a second.

Suggested: the mutation records say which window changed. Redraw that window's title state and its own tab, and redraw the docks and the taskbar only when what changed can alter them.

### 3. Two stylesheet rules make any insertion restyle the page

In `pudl.css`:

- Line 1087: `.topbar:has([data-menubar]) .status-item > .badge:is(.warn, .danger, .positive, .accent)`
- Line 984: the row actions rule that begins `:is([data-row-hot], [data-row-focus], [data-menu-for], [aria-selected="true"], [aria-current]:not([aria-current="false"]), :has(> [aria-selected="true"], > [aria-current]:not([aria-current="false"]))) > [data-row-actions]`

Each puts `:has()` in a compound other than the selector's subject. Chromium then treats text inserted anywhere in a window as able to change the match, and restyles every element the rule's right-hand side could select: every `.badge` and every row with actions on the page.

Measured: one piece of text streamed into a chat restyled about 2,800 elements. Taking out these two rules and one of Dinah's own (since replaced) left 150. With one chat open a streamed piece cost 13 ms with the rules and 6 ms without. The elimination that found them is `bench/probes/has-greedy.js` in Dinah.

Suggested for line 984: `:is([aria-selected="true"], [aria-current]:not([aria-current="false"])) ~ [data-row-actions]` selects the same button without `:has()`, because the button is the last child of its row. Suggested for line 1087, and the five rules beside it that begin `.topbar:has([data-menubar]) .status-`: an attribute or class that the menu bar script sets on `.topbar` when it finds a `[data-menubar]` inside.

### 4. The commands search the whole document on every key and every change

In `pudl-commands.js`, the `keydown` listener (line 642) runs `document.querySelector('dialog:modal')` for every key. `refreshOtherRows` (line 516) runs `document.querySelectorAll('[popover]')` and `draw` (line 587) runs `document.querySelectorAll('[data-command-choice]')` whenever the commands change.

Measured: about 2 ms for each keystroke, and 4 to 9 ms for each change of the commands, which Dinah announces whenever a chat changes state.

Suggested: keep the open modal dialog, the popovers and the choice controls in lists that are updated as they are made and removed, as `pudl-menu.js` has tracked its open panels since 0.83.2.

### 5. Work that grows with the square of the window count

`parentOf` (line 1856) reads `data-win-parent` from the DOM and searches `st.open` with `indexOf`, and `stackOf` (line 1664) calls it for every window and uses `indexOf` inside a filter over every window. `urlFor` and `windowParams` rebuild the whole address for every link in `updateLinks`. `pudlWindows.state()` deep-copies the arrangement for every caller, and most callers want only `.top`.

Measured: `getAttribute` under `parentOf` was about 18 ms of each raise, and `history.replaceState` with the long address about 6 ms.

Suggested: read each window's parent once into a map for the pass, use a set for membership, and offer a cheap accessor for the front window's key.

## What a candidate with two of them gained

A copy of 0.107.1 with only finding 1 (a kept layer width) and finding 3 (the two rules) was measured against the vendored copy in one run at 32 windows.

| Action | Vendored | Candidate |
| --- | --- | --- |
| A streamed piece of text, front window | 30 ms | 17 ms |
| A change of one window's state | 106 ms | 76 ms |
| A window brought to the front | 216 ms | 186 ms |
| Opening one more window | 1,985 ms | 1,709 ms |

Findings 2, 4 and 5 were not tried in a candidate.

## What Dinah did on its own side

These are not requests. They are here so that PUDL does not look for the same faults twice.

- A window whose content area is wholly covered by the windows above it is marked `data-dinah-covered`, and its `.win-body` gets `content-visibility: hidden` (`covered-windows.js`, with a browser suite of the same name). It reads only the DOM and `pudl:windows-change`. At 32 windows it took a keystroke from 35 ms to under 20 ms. PUDL could take the rule for itself, where it would know about drags and skins at first hand.
- Dinah stopped writing attributes with the value they already had. Such a write still produces a mutation record and a restyle, so it triggered finding 2 and finding 3 for nothing.
- Dinah stopped asking the document for `[data-win="key"]` and reads the layer's children instead.
