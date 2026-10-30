# Following chat output

Paul reported unreliable following of chat output on 24 October 2026. The chat watched changes to its log size but did not watch its output viewport. Growing the composer or opening a question could leave the latest output below the visible area. A layout-driven change to the scroll position could also be mistaken for the reader scrolling upward.

The chat now observes both the log and output viewport. It schedules tail adjustments through the shared rendering queue after writes, preserving the frame budget. Its scroll handler recognizes changes to content and viewport dimensions and retains the following state through those changes. Hidden views do not replace their scroll state with zero-height measurements.

Upward wheel and keyboard navigation, touch scrolling and scrollbar interaction pause following. Returning to the bottom resumes it. Jump to the Latest explicitly releases a pending saved position and resumes following. Normal history restoration still respects the saved reading position. Each chat retains its own following state.

The browser regression uses real overflowing content after the session has opened. It checks composer growth, question display, user scrollback during streaming, explicit resumption, late content growth and shrinking content. Against the earlier scroll implementation, composer growth and question display fail. Browser fixtures establish the layout behavior; they do not measure native input latency.
