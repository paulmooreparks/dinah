# Agent-accessible desktop diagnostics

Dinah exposes live diagnostics through the existing authenticated applet MCP service. The same page request bridge used by window commands will carry diagnostic requests. This follows `C:/Users/paul/OneDrive/Documents/Architectural Principles.md`; no second HTTP server, remote-debugging port or arbitrary JavaScript evaluator is introduced.

## Contract

The `dinah.diagnostics` provider has `snapshot@1`, `start@1` and `stop@1` actions. Snapshot is read-only and process-wide: it reports aggregate counts, timing evidence and runtime memory without document contents, identities, commands or paths. Start takes a duration of 1 to 300 seconds. Only one capture can run at once. Stop requires its capture ID; an old stop cannot terminate a newer capture. Starting a new capture clears its predecessor's retained records. Snapshot does not clear evidence.

The new `diagnostics.read` capability allows aggregate inspection. `diagnostics.control` follows the existing consent mechanism for effectful applet actions. Captures stop automatically and do not change the person's existing responsiveness-recording preference. The Settings Diagnostics pane displays capture status and lets the person stop a recording.

The frontend retains at most 200 scalar event records, with at most eight script entries per long frame. It stores no browser event objects or DOM references. Each event kind is limited to 100 records per second. Scheduler records include work taking at least 8 ms or waiting at least 16 ms. Overflow counts are explicit. Script attribution includes only paths of shipped scripts loaded by the page; other sources are classified as unknown. Browser feature support is explicit. Each snapshot includes a monotonic clock anchor, capture deadline, visibility, total DOM and transcript counts, render-queue counts, long tasks, input delay, scheduler work and long-frame attribution. The interval timer records event-loop delay; background throttling is identified by visibility and must not be interpreted as foreground blocking.

Snapshot collection measures its own cost. DOM counts are collected on demand, not on a background interval. JavaScript heap size and detached-node counts require a profiler and are reported unavailable rather than guessed. Go runtime counters describe the Go process, not the WebView renderer. They do not force garbage collection or stop the application to read a full heap dump.

The backend permits one diagnostic page request at a time and waits at most three seconds. Snapshot page requests are limited to one per second. It includes its counters even when the page is unavailable. Cancellation cleans up the request. A timed-out control request expires and must not start or stop a capture later when the renderer recovers. Requests carry an absolute expiry checked by the page.

## Reproduction workflow

An agent discovers the diagnostics actions and starts a bounded capture. It can inspect and invoke the existing workspace-scoped `dinah.windows` commands to reproduce a reported operation, subject to their normal permission rules. It then reads a snapshot and stops the capture by ID. These commands do not provide OS mouse or keyboard control, arbitrary DOM mutation, or synthetic provider messages.

Long-frame attribution uses the documented [Long Animation Frames API](https://www.w3.org/TR/long-animation-frames/). Runtime counters use Go's documented [runtime/metrics API](https://pkg.go.dev/runtime/metrics). Unsupported attribution is returned as unavailable. A capture is diagnostic evidence; successful functional tests do not establish that the overnight responsiveness blocker is resolved.

## Agent usage

Discover provider `dinah.diagnostics` through `applet_discover` and use the returned schema and digest. Invoke `dinah.diagnostics/start@1` with `{"seconds":60}`. The capture ID appears in `page.data.capture.id` when the page response is measured. Invoke `snapshot@1` with `{}` no faster than once per second while reproducing the operation. Invoke `stop@1` with `{"captureId":"the-returned-id"}` to stop early. Each applet call uses the caller's current context. A busy or unavailable page response does not establish that a control operation completed.

## Verification and build

Focused Go tests passed for diagnostics contracts, permissions, cancellation, page timeout, malformed and oversized responses, snapshot throttling, and the existing MCP and window contracts. The browser tests passed for capture lifecycle, expiry, bounded retention, source filtering, scalar copying and local cancellation. A real Chromium capture recorded an injected long task, long animation frame and slow scheduled render. The existing responsiveness snapshot and page-view suites also passed.

The first attribution test failed because its frame began before recording started. That failure remains under `build/agent-diagnostics/attribution`. The corrected test starts its workload after two animation frames; its evidence is under `build/agent-diagnostics/attribution-aligned`. The final focused run, including the per-kind rate cap and slow-render filter, passed under `build/agent-diagnostics/verified`.

The executable is `build/agent-diagnostics/dinah-desktop.exe`, version `0.0.0-agent-diagnostics-20261106`. Its hash is recorded in `build/agent-diagnostics/build-receipt.json`. This build was installed on 6 November 2026. The installed hash matches the build receipt, and the previous executable is preserved for rollback. The installation receipt is `build/agent-diagnostics/install-receipt.json`. Dinah was left running for the person to restart. These tests establish diagnostic behavior, not an improvement in overnight responsiveness. Renderer heap profiling remains outside this API.

