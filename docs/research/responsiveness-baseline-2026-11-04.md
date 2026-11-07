# Responsiveness baseline, 4 November 2026

Paul supplied this timing snapshot in chat after restarting the performance build. This file records a summary of the supplied measurements; it does not contain the full raw snapshot.

The browser capture runs from its anchor at 08:39:36.851 UTC to 08:40:22.153 UTC, approximately 45.3 seconds. It retains 46 time slots, with no overwritten slots or suspended periods. It reports 356 input raw-buffer wrap omissions, so the retained raw inputs do not cover the full interval. No work or frame wrap omissions are reported.

| Measurement | Observed value |
| --- | --- |
| Maximum input delay across browser slots | 44.7 ms |
| Long tasks | 2, lasting 53 and 54 ms |
| Long animation frames | 2, maximum 80.3 ms |
| Maximum instrumented render work | 21.8 ms |
| Maximum instrumented render wait | 29.2 ms |
| Copy operation | 11.4 ms |

The long tasks and frames occur early in the capture. Most later active input slots have maxima around 1 to 2 ms, with several around 10 ms. This capture does not contain the earlier multi-hundred-millisecond browser stalls, but the workloads differ, so it cannot establish the amount of improvement caused by the selector change. Process-wide runtime measurements cannot identify the cause of an individual stall. Slot aggregates do not establish input percentiles.

## PUDL skin observer finding

Paul subsequently supplied [PUDL issue 33](https://github.com/paulmooreparks/pudl/issues/33). The issue describes a feedback loop in yavc.app: diagram insertion triggers the PUDL skin observer, which rewrites unchanged root attributes, which triggers another diagram redraw.

Inspection confirmed that Dinah's bundled `pudl-skins.js` also calls `apply()` after document child-list changes and unconditionally writes `data-skin` and `data-skin-pref`. This establishes the presence of the reported trigger, but does not establish a redraw loop or stall attribution in Dinah. No skin code was changed in this worktree. Paul offered to have the PUDL lead handle the upstream issue.
