# UI responsiveness

Paul requested an application-wide scheduling architecture on 24 October 2026. Incoming data should update state and invalidate views; application rendering should pass through bounded scheduled work. Native typing, selection, scrolling and focus must remain usable during streaming and restoration. An FPS display is not required.

The target contract requires bounded individual tasks, visibility-aware coalescing, cancellation on disposal, and bounded visible representations of large data. A frame budget cannot preempt JavaScript already running. Moving an unbounded function into the scheduler does not satisfy this contract. Expensive transformations should run outside the UI thread or operate on bounded input. Provider event ordering, replay completion and unsent drafts must survive any migration.

Before this change, the scheduler covered streamed chat text and tools and several live panes. Provider event handlers and saved-history reconstruction now run through it; other applets still contain direct updates. PUDL and editor libraries also own interactive updates. This document does not certify that every application update is scheduled. Migration must preserve each owner's lifecycle and requires regression evidence.

Diagnostics are optional and off by default. They retain at most 200 timing records from the last minute in memory, without message text, typed content, file paths or session identifiers. Browser Event Timing reports input processing delay where supported. Long Tasks reports main-thread tasks of at least 50 milliseconds where supported. Scheduler records identify generic job categories, execution duration and queue delay; hidden time is excluded from visible queue-delay measurements. Unsupported browser measurements are shown as unavailable. Turning diagnostics off disconnects observers, stops display refresh and clears recorded samples.

The diagnostic is an observer of responsiveness, not a guarantee against browser, OS, GPU or garbage-collection stalls. Hidden/minimized windows are excluded. It cannot update its display during a main-thread stall, but can report the stall afterward. API contracts are documented by [Event Timing](https://developer.mozilla.org/en-US/docs/Web/API/PerformanceEventTiming), [Long Tasks](https://developer.mozilla.org/en-US/docs/Web/API/PerformanceLongTaskTiming) and [requestAnimationFrame](https://developer.mozilla.org/en-US/docs/Web/API/Window/requestAnimationFrame).

## Implemented boundaries

Provider events enter one ordered scheduler queue. Each job processes at most 128 updates and yields at the frame deadline. Batched updates are unpacked incrementally, and session reopening waits for preceding queued events before finishing replay. Saved and continued-history reconstruction creates at most 16 entries per scheduler invocation, also respecting the deadline, and reconnection waits for this reconstruction. Queueing does not discard provider events.

A single message over 16,384 UTF-16 code units uses a bounded plain-text page with First, Previous, Next and Latest controls. Page boundaries avoid splitting surrogate pairs. It follows the last page until the reader navigates backward. All text remains in the message model for copying and context. This sacrifices rich formatting of oversized messages to bound parsing and DOM construction. Tool output retains its existing bounded, ANSI-formatted preview and link to the full log.

The diagnostic lives in Settings > Diagnostics > UI Responsiveness. Its saved preference uses the existing UI state store. Rendering samples are retained only when a task exceeds 6 ms or waits at least 16 ms; browser samples use their documented thresholds. At most 20 recent records are displayed. Timing categories contain no conversation identifiers.

## Remaining migration

This change does not complete the application-wide contract. RPC-result handlers, some applet redraws and final layout callbacks still need individual migration and measurement. JSON decoding, string accumulation, trimming old rows and third-party diagram/editor rendering can still consume main-thread time. The provider queue currently preserves every event and has no transport backpressure. Sustained traffic beyond consumption capacity can build a backlog. Further work must add bounded representations and cooperative processing at these boundaries without dropping history, reordering lifecycle events or changing native editor semantics.

## Settled history restoration

Saved history and provider replay remain hidden while their bounded rendering jobs run. A completed render swaps the replacement into the chat, positions it, and reveals it within the same task. Quiet reconnection retains the saved conversation throughout. Tail scheduling is suppressed during loading and settling. When a reader scrolls the saved conversation during reconnection, replacement preserves its scroll offset rather than forcing the end. Failed reconnection retains the saved copy. This avoids intermediate visible replay and repeated tail layouts; it does not eliminate the final layout cost.

Chat Markdown recognizes workspace-relative and absolute local links, including angle-bracket paths with spaces. The owning chat execution folder resolves relative paths through the native file-opening policy. Sent raster images render bounded thumbnails and open through the existing image viewer. Prompt and replay capture retain image blocks for saved-history display. Handoffs remain text-only and continue to disclose binary omissions. Older text-only saved records cannot recover omitted image bytes.

## Restore and resize investigation, 24 October 2026

Paul authorized coordinated investigation with PUDL Development. PUDL owns its ribbon and outline-resize work. Dinah owns transcript rendering and scroll persistence. No PUDL source or vendored files were changed for these application experiments.

The benchmark used an authenticated hidden instance of installed Dinah c45ba6c with an isolated copy of native state and real SavedConversation calls. Edge rendered four quarter windows using the four largest available saved histories, containing 6,571, 4,878, 4,378 and 2,293 entries. The resulting page contained 21,034 activity rows and about 177,397 DOM nodes. Provider opening was blocked, and continuation lookup was pinned to each selected saved identity, avoiding concurrent provider writers. These are measurements of the authenticated HTTP equivalent, not native WebView2 composition or provider startup. The profiler harness is in `cmd/dinah-desktop/frontend/e2e/perf/restore-resize.cjs`.

Two alternating unprofiled baseline runs restored in 13.729 and 13.956 seconds. The experimental message content-visibility runs restored in 13.908 and 14.718 seconds, so this experiment did not establish a restore improvement. Over 24 viewport sizes, baseline resize sequences took 6.183 and 6.344 seconds; the candidate took 3.314 and 3.719 seconds. Layout duration fell from 1.182/1.145 seconds to 0.231/0.237 seconds. These small samples establish a useful resize signal, without proving production latency or a precise percentage improvement.

Tracing attributed roughly 7.5 seconds of a profiled restore to style updates, 4.7 seconds to script and 0.5 seconds to layout. Paint events totaled about 0.28 seconds. A profiled resize sequence spent about 1.39 seconds in layout, 0.95 seconds in prepaint and 0.29 seconds in paint. Content visibility reduced layout and prepaint substantially; paint did not consistently improve. CPU sampling also exposed native querySelector calls during resize; caller attribution remains work for the coordinated PUDL investigation.

Native bridge request timing was recorded separately. Four SavedConversation responses totaled about 0.57 to 0.79 seconds of request duration in a later comparison. SessionsAt calls overlapped and individually reached about 1.3 to 1.5 seconds. Their summed durations must not be added to browser wall time. These timings include browser/network scheduling and do not isolate Go CPU. Provider reconnection, live transport bursts, native host painting and GPU composition remain unmeasured.

The scroll watcher also spent about 1.4 to 1.8 sampled seconds in workbench.js `consider`. Transcript descendants are now excluded through `data-ui-scroll-ignore`; the outer activity scroller remains watched. A candidate profile removed that sampled hotspot and lowered style duration to about 6.4 seconds, but repeated total restore times were noisy and did not establish a consistent end-to-end gain. This exclusion avoids recording incidental Markdown/code element scroll containers as personal view state.

Message rows use `content-visibility:auto` with an intrinsic block estimate of 120 pixels, based on a measured baseline median row height of about 106 pixels. The browser retains the mounted DOM and can remember rendered heights. No whole-window containment was added. Printing disables the optimization. A first experiment broke exact persisted scrollback restoration because skipped rows initially use estimates; the production rule therefore excludes logs restored into scrollback, or swapped while the reader is above the end. Jump to End clears that exclusion. This trades some resize performance for exact reading-position correctness.

Regression coverage checks retained selectable text, offscreen focus, streaming, tailing, manual scrollback, pinned navigation, saved positions, images, media viewing and steering. The initial unrestricted CSS failed the existing delayed-restoration test; the gated rule passed that test. Find availability follows the documented `auto` contract, but native browser Find UI and system clipboard interaction still need manual verification. No production performance claim is made for those interactions.

The initial harness runs that timed out before readiness, and the run with an overly broad asset-route pattern, were discarded. No timings from those invalid runs support the results above. Authentication cookies and copied conversation content stay in the isolated temporary directory and are excluded from the repository.

References: [content-visibility](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/content-visibility) and [intrinsic size](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/contain-intrinsic-size) document skipping, retained user-agent features and remembered dimensions.

The final gated application implementation passed all 71 browser suites together in 139 seconds at four concurrent suites, plus `go vet ./...`. The benchmark and documentation do not claim that startup is solved. PUDL follow-up work remains independent and unreleased.

## Isolated PUDL menu comparison, 25 October 2026

A second experiment used installed d7a3182, which already includes the application changes above. The same four histories produced 21,034 rows and 177,510 DOM nodes with the ribbon enabled and its View tab selected. Four profiled runs alternated installed assets and a replacement of only `/pudl/pudl-menu.js`. The replacement was PUDL Development's local, unreleased open-panel tracking implementation, captured with SHA-256 `f38f61a70f24acf91a5c897c3ed79ee2442b975db79b792cf99f00336c9a0734`. It was not vendored or deployed.

| Measurement across 24 viewport changes | Installed menu, two runs | Candidate menu, two runs |
| --- | --- | --- |
| Wall time | 4,800 / 4,705 ms | 3,062 / 3,112 ms |
| Script duration | 1,808 / 1,893 ms | 508 / 426 ms |
| Layout duration | 404 / 416 ms | 355 / 320 ms |
| Style duration | 882 / 842 ms | 814 / 824 ms |

CPU caller attribution identified `querySelector` from `placeOpen` in the installed menu script, accounting for about 1,206 sampled milliseconds in the first baseline resize run. The candidate removes this sampled hotspot by tracking open panels instead of searching the full document during resize and captured scroll. Remaining window and dock selector costs were substantially smaller in this capture. These results were sent to PUDL Development for its library work.

Restore wall times were 13,895 / 14,462 ms with installed assets and 13,819 / 13,555 ms with the menu candidate. Style work still consumed about 6.2 to 6.6 seconds. This comparison does not establish a startup solution. CPU profiling and tracing were enabled, and wall time includes profile-stop overhead. The experiment measures the authenticated Edge HTTP equivalent with provider opening blocked; native WebView2 composition and active streaming remain outside its evidence.

The menu candidate was subsequently committed upstream as `52af219f983242807656577373cf7af4df04e2fd`. Dinah vendors that isolated readable script in both distributions as a patch on PUDL 0.83.1. Its hash matches the tested candidate above. This is an unreleased upstream commit, and does not include the separate ribbon or outline-resize work.

PUDL subsequently released this identical menu script in [0.83.2](https://github.com/paulmooreparks/pudl/releases/tag/v0.83.2), tagged at `8957b496c0be2795caff0a152ef297a9d7c61b95`. Both Dinah menu-script copies were verified byte-for-byte against the tag. Dinah still carries the surrounding 0.83.1 distribution with this isolated released patch; this verification does not claim a wholesale 0.83.2 upgrade. Future updates must use tagged assets because the PUDL working tree now includes separate unreleased 0.84 changes.

## PUDL 0.84 integration

Dinah upgrades both vendored distributions from the authoritative `v0.84.0` tag at `4b73e193a1dc4da14b58a1471ccea94743a1e4eb`. All 60 included distribution files in each copy were imported from the tag; the local VERSION marker records 0.84.0. The tooltip correction described below is the sole subsequent vendor patch. The tracked-menu script retains its measured hash. No assets are taken from PUDL's mutable working tree.

The desktop window layer enables `data-win-resize-mode="outline"`. Floating frames, outer dock edges, shared dock panes and snapped-grid dividers preview pointer geometry while keeping their content mounted at its previous size. Release commits the geometry; Escape and pointer cancellation discard it. Keyboard resizing remains immediate. The separate bus web page retains its existing live default. Native operating-system window resizing is unaffected.

The upgraded ribbon includes full-title tips and the labeled Ribbon tabs chooser. Desktop already loads `pudl-tooltip.js`. The application regression checks the outline preview's fixed content geometry, release geometry, keyboard opening of the chooser and focus return after Escape. These integration checks complement upstream lifecycle and cross-engine tests; they do not measure native resize latency or resolve restoration's style/script costs.

The initial full browser run passed 70 of 71 suites. The keyboard regression failed repeatedly because full-title focus tips made the existing tooltip Escape handler intercept the menu bar's Escape navigation. Both vendored `pudl-tooltip.js` copies now dismiss the tip without stopping key propagation. The existing keyboard assertion verifies that one Escape returns from the bar to the editor. This is a Dinah vendor patch beyond the 0.84.0 tag; the remaining 59 imported files in each distribution stay byte-identical to it. The patch does not change focus itself or alter the tooltip's dismissal behavior.

The final patched bundle passed all 71 Dinah browser suites together in 142 seconds with four concurrent suites, plus vet and the web-package Go tests. Screenshot review covered the ribbon chooser, and the outline preview/release checks passed in the application workbench. These checks establish integration behavior, without adding a new performance benchmark or native interaction claim.

## PUDL 0.85.0 follow-up

Both distributions now use the complete 0.85.0 tag at `2cc2394df80d7b979861d7efca9524f23b5cb178`. The earlier local tooltip and menubar-focus patches are superseded by upstream implementations. The tagged menu performance correction remains included. This dependency upgrade establishes no additional startup or native WebView2 performance result.
