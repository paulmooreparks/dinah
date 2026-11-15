# Diagram pictures for a product's tools program

This is the proposed protocol 1 contract for a tools program to ask its running window for a new picture of a Canvas or Mermaid file. It adds no publication authority. The product adapter must obtain every required new picture successfully before its first upload. Existing D2 rendering and reuse of an already verified picture retain their current behavior and do not require this lease. The architectural principles in `C:/Users/paul/OneDrive/Documents/Architectural Principles.md` apply.

The current window already calls `yavcDiagramPicture.draw` from `frontend/yavc/content/diagram-kit/picture.js` through `frontend/diagram-picture.js`. That path uses the main page and its cooperative page-ask cancellation. The helper service must use a separate disposable native page so a stuck drawing cannot be left running on the main page. No new diagram renderer, external browser, browser debugging port or headless browser is introduced.

## Authority and discovery

Only a product tools program named by `profile.ToolServers` may use this service. A new explicit `DiagramPictures` permission on a tool-server definition defaults to false. The host supplies a short-lived, random lease bound to its window instance, one registered workspace and the permitted tools executable. A normal applet context, bus identity, release string or executable basename does not grant this permission. The service is absent from the agent-facing applet and bus tool catalogs.

The launcher supplies the endpoint and lease to its workspace's managed tools registration. The lease is never accepted from an agent's tool arguments, written to disk or logged. A helper launched outside a window without a workspace-bound lease refuses only requests for new Canvas or Mermaid pictures; it does not guess a workspace from its working directory or ask for a general home token. Closing the workspace or window revokes its leases and cancels its requests. This is an additional optional tool-server launch capability; existing registrations continue without it.

The workspace-bound bearer lease is the authority, as with existing helper capabilities. The account and its process memory are trusted. Protocol 1 performs no owner-PID or executable-identity check; a later protocol may add caller identity if a concrete threat requires it.

The endpoint binds only to `127.0.0.1`, rejects unexpected Host and Origin values, offers no CORS access and accepts no cookie authentication. Authorization is checked on every request. The protocol uses `X-Helper-Channel: diagram-picture`, `X-Helper-Protocols` and `X-Helper-Protocol`, following `docs/design/helper-compatibility.md`. Both peers offer numbers and select the highest common number before body decoding or work. A missing offer means legacy 1, but does not create a service on an older window. No common number gives the existing restart refusal, with the newer side identified. An older window's missing route gives `picture_service_unavailable`, never permission to upload without pictures.

If no window answers discovery within one second, the client returns `window_unavailable`: "A new Canvas or Mermaid picture needs a managed chat in the relevant workspace inside the product window. Publish through that chat. Nothing was uploaded." A missing lease gives `picture_lease_unavailable` with the same next step. Merely opening a window gives an existing outside process no lease. This service never starts or brings forward a window.

### Delivery when another registration already exists

The window must not depend on duplicate suppression to deliver the lease. It preserves the person's existing registration, arguments, environment and approval settings. Instead, it offers a distinct, session-scoped managed server alias for the installed tools program, with a fresh lease under `DIAGRAM_PICTURE_LEASE`, endpoint under `DIAGRAM_PICTURE_ENDPOINT` workspace identity under `DIAGRAM_PICTURE_WORKSPACE` and the window-resolved absolute workspace root under `DIAGRAM_PICTURE_ROOT`. The root is information for snapshotting and grants no authority. Requests remain workspace-relative; the window confines and hashes each path against its own resolved root. The alias is selected after inspecting the effective names, and a conflict refuses capability activation rather than overriding a custom entry. The chat identifies which managed server can draw new pictures; the existing server remains available for its existing operations. Product adapters own the tool presentation under that alias.

The extra managed server offers a restricted catalog containing only publication operations that need drawing. It must never expose a second full tools catalog. Those publication operations retain their existing write classification, approval and authorization checks; the drawing lease grants no publication authority. The product adapter must test that catalog restriction and the unchanged publication checks.

| Agent program | Documented delivery mechanism and refusal boundary |
| --- | --- |
| Codex app-server | Supply a distinct server in the thread's runtime config, with the documented stdio `command`, `args` and `env` map. Check effective configuration first. Do not modify an existing server or rely on recursive same-name merging. Redact the lease from protocol traces before serialization. |
| Claude Code | Supply the distinct server through inline JSON in `--mcp-config`, with its stdio `env` map. Preserve all existing configured servers; do not use `--strict-mcp-config` to hide them. Do not write a temporary JSON file containing the lease. Redact launch and protocol traces. |
| Devin CLI | The reviewed documentation establishes persisted MCP configuration and `env` fields, but does not establish an in-memory, per-session injection mechanism that meets the no-written-lease rule. Protocol 1 does not grant the capability through Devin. The chat must report that new Canvas and Mermaid pictures require a supported managed provider in the window. Existing tools, D2 and verified-picture reuse continue. Persistent JSONC must not be rewritten to carry a lease. |
| Ollama | Dinah owns the MCP client and process launcher. Start the distinct managed server with the environment map directly, using the existing host-owned session configuration; no external provider configuration or environment inheritance assumption is needed. |

These are implementation requirements, not a claim that launch adapters already carry leases. The Codex and Claude paths require stand-in protocol tests with a custom same-name registration present, showing the extra managed alias receives the lease while the original entry is unchanged. Resumed or already running sessions must establish a new supported managed registration before drawing, or report the limitation visibly; no silent fallback grants an outside server a lease. No provider is assumed to forward arbitrary parent environment variables.

Support in this iteration is Codex and Claude through their documented per-session configuration, and Ollama through the window's own process settings. Devin is unsupported for new Canvas and Mermaid pictures. Unsupported users are directed to a supported chat inside the relevant workspace in the window, or to publishing through the window itself.

## Requests and results

The host exposes these authenticated routes. IDs are random client-generated UUIDs, scoped to the lease; a duplicate active ID is refused with 409. Terminal IDs are retained for two minutes to refuse replay, bounded to 256 per lease, after which that lease must be renewed.

| Route | Meaning |
| --- | --- |
| `OPTIONS /diagram-pictures` | Negotiates the channel and returns 204 without decoding a drawing request. |
| `POST /diagram-pictures` | Validates a snapshot, draws it and returns one terminal result. |
| `DELETE /diagram-pictures/{requestId}` | Cancels only this lease's request and discards its page; repeated cancellation returns 204. |

The POST JSON shape is frozen as follows. Paths are workspace-relative file paths, with `/` separators. No absolute path, URL, home mount, `..`, alternate data stream or other workspace mount is accepted.

```json
{
  "requestId": "uuid",
  "workspaceId": "registered-workspace-id",
  "source": {"path": "diagrams/flow.canvas", "sha256": "lowercase-hex"},
  "pictures": [{"path": "diagrams/images/logo.png", "sha256": "lowercase-hex"}],
  "format": "png",
  "scale": 2
}
```

`workspaceId` must equal the lease's workspace. The host resolves each file through that workspace's file authority, including final link resolution and permissions. It reads each file once through an authorized handle, hashes those bytes and compares them to the request. The in-memory bytes are the drawing snapshot; paths are never read again by the page. The request does not accept caller-supplied source text or replacement file contents. A missing file or hash mismatch refuses the whole request. This guarantees which bytes were drawn even if the file changes afterwards; it does not lock files until publication.

The source extension selects Canvas (`.canvas`) or Mermaid (`.mmd` or `.mermaid`). It must contain valid UTF-8. `format` is required and is `png` in protocol 1; `scale` is required and is 1, 2 or 3. The complete `pictures` list declares local image dependencies. Canvas references are resolved relative to the source folder to precisely one declared workspace path, with no basename fallback. Missing declarations, unsupported image types or remote resources refuse. Accepted dependencies are PNG, JPEG and WebP. The host passes only the verified image bytes to the picture function. Dependencies cannot load through arbitrary network or file URLs.

Successful HTTP 200 returns this JSON. `pictures` repeats every dependency's verified path and hash in request order. The host verifies the PNG signature, dimensions and byte limit before returning it.

```json
{
  "requestId": "uuid",
  "workspaceId": "registered-workspace-id",
  "source": {"path": "diagrams/flow.canvas", "sha256": "lowercase-hex"},
  "pictures": [{"path": "diagrams/images/logo.png", "sha256": "lowercase-hex"}],
  "mime": "image/png",
  "data": "base64",
  "sha256": "picture-bytes-lowercase-hex",
  "width": 1600,
  "height": 900,
  "problems": []
}
```

Any nonempty `problems` result from yavc.app is a failure, even if a picture was produced. Failure returns no picture bytes, with `{requestId,code,message,problems}`. HTTP status is 400 for malformed input, 401/403 for authority, 409 for snapshot conflict or replay, 413 for limits, 422 for a drawing problem, 429 for capacity and 503 for unavailable native rendering. Deadline expiry returns 504. Cancellation returns a terminal `cancelled` failure if the original connection remains open. A mismatch, malformed response, problem, timeout, unavailable service or cancellation must prevent publication. The client validates the repeated workspace, ID and entire snapshot identity before accepting bytes.

The generic Go client contract is `Draw(ctx context.Context, request Request) (Picture, error)` in a new `internal/diagrampicture` package. It owns discovery, negotiation, response limits and transport cancellation. `Request`, `Picture` and typed refusal fields map to the JSON above. The workspace lease comes from client construction, not from `Request`.

The exported Go surface is frozen as follows. `New` validates its inputs without connecting; `Draw` performs the authenticated negotiation and bounded request. `Data` is decoded PNG bytes; JSON represents it as base64. Refusals support `errors.As`. A transport with no answering window becomes a typed `window_unavailable` refusal. The client never starts a window or reads credentials.

```go
func New(endpoint, lease, workspaceID string) (*Client, error)
func (c *Client) Draw(ctx context.Context, request Request) (Picture, error)
type Snapshot struct { Path string; SHA256 string }
type Request struct {
    RequestID string
    WorkspaceID string
    Source Snapshot
    Pictures []Snapshot
    Format string
    Scale int
}
type Picture struct {
    RequestID string
    WorkspaceID string
    Source Snapshot
    Pictures []Snapshot
    MIME string
    Data []byte
    SHA256 string
    Width int
    Height int
    Problems []string
}
type Refusal struct {
    RequestID string
    Code string
    Message string
    Problems []string
    HTTPStatus int
}
func (r *Refusal) Error() string
```

Field JSON names follow the wire examples above; `HTTPStatus` is local diagnostic data and is not sent as a refusal-body field. `Source`, `Pictures` and `SHA256` must all be validated against their byte snapshots before accepting a result.

Before requesting a new picture, the adapter retains the source and image bytes whose hashes it supplies. After validating the returned identities, it uploads those retained source and dependency bytes and the returned picture bytes. It must never reopen their paths between drawing and upload. A mutation-between-drawing-and-upload test must show that the uploaded bytes still equal the retained snapshots and hashes. The adapter owns publication permissions and must refuse the entire publication before uploads if a required new picture is unavailable. This refusal does not prohibit D2 rendering or reuse of an already verified picture under existing checks.

## Bounds and page disposal

| Resource | Protocol 1 limit |
| --- | --- |
| Request JSON | 64 KiB, including at most 32 image dependencies |
| Diagram source | 2 MiB of UTF-8 bytes |
| Each image dependency | 1 MiB, matching yavc.app's Canvas reader |
| All dependency bytes together | 8 MiB |
| Returned PNG | 16 MiB decoded; 24 MiB maximum response body |
| Picture dimensions | 8,192 pixels per side and 16,777,216 pixels in total |
| Drawing lifetime | 30 seconds from admission, including startup and dependency reads |
| Capacity | One active page per workspace and two per window; excess work is refused immediately |

The host also validates each dependency's decoded header before entering the renderer. Each image obeys the same 8,192-pixel side and 16,777,216-pixel area bounds as the result. This prevents a small compressed file from requesting an unbounded bitmap allocation. A lease lasts fifteen minutes; the provider must establish a fresh managed registration when it renews the lease. Neither lease expiry nor replay-budget exhaustion falls back to an unrestricted registration.

The client may impose a shorter context deadline. Host admission starts the watchdog independently of page JavaScript. The host creates one dedicated native embedded page per request, loads only its bundled drawing assets and product fonts, and calls yavc.app's existing picture function. Its document contains no applet UI, PUDL window, agent bridge, user profile storage or general file bridge. Navigation, downloads, popups and external resource requests are refused. Assets and verified request bytes are available only under this request's private route, which is revoked at termination. The source's text is passed as data rather than executable script.

Success, failure, cancellation, client disconnect, workspace close and timeout all revoke the request and close the whole native page controller. `AbortSignal` is a courtesy to cooperative code and is never the disposal mechanism. No terminal request can publish a late callback. A completed page is not reused. The watchdog must be able to invoke native disposal while the renderer is stuck in JavaScript; it cannot rely on JavaScript timers, `window.close`, navigation completion or a response from the page.

On Windows the native backend uses WebView2 controller lifetime and explicit `ICoreWebView2Controller.Close`. The controller is hidden before its first navigation, and its parent is never shown or activated. It never calls a focus method. The existing applet/page-ask path is insufficient because it has no independent controller to close. Closing the controller is required before releasing the capacity slot; a disposal failure disables further drawing and returns `renderer_unavailable` rather than treating a leaked page as finished.

### Unconfirmed startup disposal

The design owner decided this exception on 15 November 2026 (message 8346). WebView2 creates environments and controllers asynchronously, with no documented cancellation operation before the completion handler provides a controller. At the admission deadline, an unresolved startup returns `renderer_unavailable` with no bytes and disables new drawing throughout the window. The apartment and callback state remain owned until the documented completion handler arrives; they must never be released while native code can still call them.

The refusal says: "Drawing is unavailable until the earlier page has finished starting, or until the window restarts. Nothing was uploaded." When the late handler delivers a controller, the apartment calls Close immediately, without navigation or drawing. A failed creation result is released. Successful disposal restores admission after every unresolved startup has settled. Close failure leaves drawing disabled under the permanent-disposal-failure rule. If a handler never arrives, drawing remains unavailable until the window restarts. This exception permits a bounded refusal while startup disposal remains unconfirmed; it never permits another page to be admitted in that window meanwhile.

The native test-only seam must delay completion-handler delivery while retaining the native references. Acceptance covers a late handler followed by a fresh successful draw, continued refusal while a handler remains withheld, and permanent refusal after an injected Close failure. A fixture may release a never-arriving handler during its final cleanup, after the refusal assertion, to avoid leaving test processes behind. No production route or tool argument controls either seam.

The correctness basis is the documented web platform described below. A native test confirms that the integration follows that argument; it cannot authorize reliance on undocumented behavior. If the wrapper cannot supply the controller lifecycle, the service remains unavailable and the missing capability is reported. No headless-browser fallback is authorized.

## Hidden-document execution and source audit

A hidden document must remain fully active with scripting enabled. Being hidden is distinct from being detached, frozen, suspended or discarded. The HTML event-loop processing model still services ordinary tasks and microtasks for a fully active hidden document, while its rendering steps filter hidden documents out. Rendering opportunities and animation-frame callbacks therefore cannot be progress dependencies. Timers may be throttled and cannot define a readiness barrier or the service deadline.

Geometry and drawing must use synchronous, documented operations on elements with layout boxes: SVG `getBBox`, CSSOM `getBoundingClientRect`, and canvas `measureText` and bitmap drawing. The drawing stage is attached with a specified width and `visibility:hidden`, never `display:none`. Its size comes from explicit diagram geometry and synchronous measurements, never from a visible viewport, a screenshot, paint completion or an animation. Product fonts must be explicitly loaded through the font-loading API before measurement. The host must refuse if the required 2D canvas context is unavailable, rather than accepting the Canvas library's non-browser estimated-text fallback.

Asynchronous asset loading, image decoding and canvas serialization use their documented load/error and completion tasks and promises. They do not wait for a paint. No `requestAnimationFrame`, `requestIdleCallback`, animation event, resize-observer delivery or timer-based settling may gate a returned picture. The native watchdog still bounds slow or throttled work. A renderer upgrade must preserve this property; any required frame or paint dependency is a critical request to yavc.app under Paul's documented-behavior rule. The host must not make its window visible to satisfy it.

The source audit is against this branch's vendored yavc.app. `picture.js` has SHA-256 `ad1c7312b02fde3311b70b935f4628a136622342fe163c3b7db8c7955be1eae7`, `md-editor/kit.js` has `28205f9aeede506d13b3f30685c3c5812140ad0f79c28bd585f1395c42b04975`, and bundled `mermaid.min.js` has `581ed7d74bd9048d0e3a91363927d72ef22942d7722546b27f7cc29e35390eb8`. No renderer files were changed and no native measurement was used as its premise.

| Path inspected | Finding |
| --- | --- |
| `diagram-kit/picture.js`, `canvas`, `mermaid` and `png` | Canvas returns SVG from its geometry renderer. Mermaid calls `drawMermaid`, then reads explicit SVG viewBox geometry or `getBoundingClientRect`. PNG uses an image's load/error event, canvas `drawImage` and `toBlob` completion. There is no frame, animation or paint wait. |
| `diagram-kit/jcanvas.js`, `measurer` | Text width and font metrics use synchronous canvas `measureText`. The library can estimate when no 2D context exists; service admission must prohibit that fallback. |
| `diagram-kit/canvas-pictures.js`, `collect` | Verified bytes are accepted directly, with a 1 MiB per-image and 8 MiB total limit. Its 10-second timer only aborts dependency reading; it is not a successful-read prerequisite. The service supplies bytes and owns the actual deadline outside JavaScript. |
| `md-editor/kit.js`, `drawDiagram` | The 400 ms loading-indicator timer is cosmetic and is cleared on completion. Readiness is the Mermaid/sanitizer load promises followed by `mermaid.render`, not that timer. `startOnLoad:false` avoids automatic page scanning. Sanitized SVG insertion and later geometry reads are synchronous. |
| Bundled Mermaid, `tu.measure` and `QXe` initialization | The measurement scheduler extends FastDom with `raf(e){queueMicrotask(e)}` on this web platform. Its fallback uses a zero-delay timer only where queueMicrotask is absent. The service requires queueMicrotask support; it does not assume FastDom's default rAF scheduler runs while hidden. All seven `tu.measure` uses read SVG boxes or CSS rectangles. |
| Bundled Mermaid, `YTe` and `VDt` layout entry points | The Cose-Bilkent and fCoSE layouts explicitly use `animate:false`; returned geometry is based on computation completion. The bundle also contains D3 transition and Cytoscape animation-frame machinery. These background/interactive animation callbacks are not awaited by these static layout completion paths. They must not be promoted to readiness signals. |

Mermaid's HTML-label image helper also registers image load/error handlers and schedules a zero-delay check of the image's `complete` state. Its parser infrastructure can yield through ordinary timer tasks. These are ordinary task scheduling, not a wait for a frame or a chosen delay intended to settle layout; hidden throttling can still exhaust the native deadline and must then produce a refusal. No elapsed timer is evidence that a picture or layout is ready. The service's external-image refusal prevents this helper from fetching undeclared remote images.

The audit found no awaited frame or paint in these picture completion paths. It does not claim that searching for a token alone proves reachability, or that an arbitrary future Mermaid plugin is safe. Protocol 1 loads only this bundled renderer and its audited built-in static layouts. Any additional plugin or layout needs the same audit before enablement. Native cases must cover the chosen diagram families, exercise frame callbacks that never fire and cosmetic timers that never fire, and still obtain a valid picture or a bounded explicit refusal. They must separately confirm native disposal of an actually stuck script.

## Required verification before enabling the service

- Tests must refuse missing, wrong-workspace, expired and ordinary applet credentials, and keep leases out of saved files, launch logs and protocol traces.
- Tests must cover equal, older, newer, absent and malformed protocol offers, a missing route and no answering window, with zero upload calls after refusal.
- Tests must show exact source and dependency hashes repeated after file mutation, refuse a wrong hash, enforce final-path confinement and reject undeclared and remote images.
- Tests must exercise source, request, image, result, dimension, deadline and capacity limits, and treat a nonempty problems list as failure without returned bytes.
- Native hidden tests must draw Canvas and Mermaid fixtures using the bundled picture function, preserve foreground focus and create no visible window.
- A deliberately stuck script must be cancelled and timed out by closing its native page. Tests must observe disposal, reject late callbacks and draw successfully in a fresh page afterwards.
- Tests must preserve a custom tools registration while delivering a lease to a distinct managed alias through each supported provider's documented mechanism, and show a visible refusal for unsupported delivery.
- Adapter tests must mutate source and dependency files after drawing and verify that uploads use the retained snapshot bytes, with D2 and verified-picture reuse unchanged when no drawing lease exists.

The native acceptance seam is compiled only with `-tags native_drawing` and lives in `internal/drawingpage`'s `_test.go` fixture. The fixture calls the private native page runner with a script that posts `started` through WebView2's own message event, then runs `while (true) {}`. The production exported API accepts only diagram input, never arbitrary scripts. There is no test HTTP route, debugger access or production test switch. The fixture waits for that started event before cancellation, and confirms the timeout case also reached it. Lifecycle observations report `controller-created`, `controller-hidden`, `script-started`, `controller-closed` and `closed-controller-refused`. The final observation requires a native method on the closed controller to fail. It then draws in a newly created controller and checks the resulting PNG bytes. Native acceptance must also check the foreground window and absence of visible process windows throughout the run.

## Documented platform dependencies

The native lifecycle depends on [ICoreWebView2Controller](https://learn.microsoft.com/en-us/microsoft-edge/webview2/reference/win32/icorewebview2controller), which documents visibility and synchronous Close, including stopping handlers without beforeunload. The web execution argument uses the [HTML event loop](https://html.spec.whatwg.org/multipage/webappapis.html#event-loop-processing-model), [microtasks and timers](https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html), [canvas text measurement and bitmap operations](https://html.spec.whatwg.org/multipage/canvas.html), [CSSOM element geometry](https://drafts.csswg.org/cssom-view/#dom-element-getboundingclientrect), [explicit font loading](https://drafts.csswg.org/css-font-loading/#font-face-load) and [SVG getBBox](https://svgwg.org/svg2-draft/types.html#__svg__SVGGraphicsElement__getBBox). The contract needs implementation and native acceptance before a product may advertise it.

Provider delivery is grounded in the official OpenAI documentation for [Codex MCP stdio environment maps](https://developers.openai.com/codex/mcp/) and [app-server runtime thread configuration](https://developers.openai.com/codex/app-server/), Claude Code's [MCP configuration](https://code.claude.com/docs/en/mcp) and [inline --mcp-config option](https://code.claude.com/docs/en/cli-reference), and Devin's [persisted MCP configuration](https://docs.devin.ai/cli/extensibility/mcp/configuration). Devin's documented persisted configuration is not permission to persist this lease.

The provider acceptance checks require `DIAGRAM_PICTURE_ROOT` to reach the managed server for Codex, Claude and Ollama alongside the workspace identity. A product adapter supplies its restricted publication catalog; this adds no publication authority or approval grant.
