# Disposable drawing page native check

The native candidate is on `engine/drawing-page`, based on master `649016a1`. It implements the first stage of the diagram-picture contract at `c1082aaf`. It adds no route, lease, provider registration or tools client.

## Native lifetime

Each draw creates a WebView2 environment and controller in its own COM apartment. The controller uses `HWND_MESSAGE`, which Microsoft's environment interface documents as an invisible WebView parent, and sets `IsVisible` to false before navigation. The page loads the bundled drawing assets through an intercepted synthetic origin. It calls yavc's `picture.js`; it does not introduce another renderer.

The native watchdog posts to the apartment's message queue independently of JavaScript. Success, a drawing error, cancellation and timeout all call `Controller.Close` before returning. The implementation releases the native references afterwards. It creates another controller for the next request. Microsoft's documented Close operation is the disposal guarantee. The fixture also observes that script admission through the closed controller fails.

The drawing path obtains SVG geometry before allocating a PNG and refuses oversized dimensions. The returned PNG is fully decoded and checked against its declared geometry and byte limit. The contract's source audit supplies the argument for hidden execution and scheduling; these native checks confirm the fixtures on this machine. They do not replace that audit.

The API documentation is at:

- https://learn.microsoft.com/en-us/microsoft-edge/webview2/reference/win32/icorewebview2environment
- https://learn.microsoft.com/en-us/microsoft-edge/webview2/reference/win32/icorewebview2controller

## Results

The final tagged run passed on 15 November 2026 in 7.920 seconds. Its evidence folder is `%TEMP%\drawing-page-20e7050a38da4034b1180414c72582b9`. The folder contains the PNGs, per-case lifecycle events and separate private browser stores.

| Case | Observed result |
| --- | --- |
| Canvas | A valid 288 by 128 PNG was returned and the controller closed. |
| Mermaid | A valid 283 by 70 PNG was returned and the controller closed. |
| Mermaid with frame callbacks disabled | The same dimensions were returned without invoking requestAnimationFrame callbacks. |
| Invalid Mermaid | The parser error was returned and the controller closed. |
| Cancellation during an infinite script | The fixture observed script entry, cancellation and native closure. |
| Watchdog during an infinite script | The fixture observed script entry, deadline expiry and native closure. |
| Fresh page after the stuck pages | A valid Canvas PNG was returned from a new controller. |

The fixture sampled foreground focus and enumerated visible top-level windows every five milliseconds. It observed neither a focus change nor a visible top-level window owned by the test process. The native construction itself uses the documented invisible parent and never creates a top-level host window.

The full desktop package also passed, taking 254.072 seconds. The ordinary drawingpage package passed with it. The final tagged run includes the later SVG dimension preflight, PNG corruption checks and dependency reproduction.

To repeat the native fixture, set `DRAWING_PAGE_EVIDENCE` to a fresh temporary folder and run this command through the shared test slot:

```powershell
pwsh -NoProfile -File scripts/with-test-lock.ps1 go test -tags native_drawing ./internal/drawingpage -count=1 -v -timeout=120s
```

The stuck-script entry point exists only in `_test.go`. It invokes the package's private runner with a script that posts a started message and enters `while(true){}`. Production callers receive only the typed diagram API. There is no product test route or script parameter.

## Dependency defect and local binding

`github.com/wailsapp/go-webview2 v1.0.22` loses nonempty response bodies in `ICoreWebView2Environment.CreateWebResourceResponse`. The wrapper declares an outer `stream`, then uses `stream, err :=` inside the nonempty-content block. The inner variable receives the memory stream, but the native call receives the outer zero value. This left every bundled script empty in the first two fixture runs.

`response_windows.go` is the small removable binding approved by the lead. It calls documented `SHCreateMemStream` and the documented environment response method with the same stream. Its comment names the dependency version and shadowed variable. No dependency source was edited.

An upstream maintainer can reproduce the defect without navigation:

1. Initialize a COM apartment and create a WebView2 environment.
2. Call the dependency's `CreateWebResourceResponse` with `[]byte("body-marker")` and status 200.
3. Read the response's documented Content property. It is null, despite the nonempty input.
4. Repeat with the local binding. Content contains the eleven bytes `body-marker`.

`TestNativeResponseBody` performs these steps and passed. It intentionally reports a failure if the dependency starts preserving the body, so the workaround can then be removed. No upstream issue has been filed.

The source search found no other direct call in Dinah. Wails v2.11.0's asset server calls this wrapper with nil content in `pkg/assetserver/webview/request_windows.go`, then supplies bytes through `PutByteContent` in `responsewriter_windows.go`. That path avoids the defective branch.

## Remaining integration gates

This candidate proves the native fixtures requested for stage one. Quality's independent fixture binding and run remain pending. The service stages still need workspace snapshots, verified image dependencies, explicit product font loading, capacity and replay checks, and provider registration. Before exposing the page through routes, the integration must also cover cancellation during asynchronous startup and explicit navigation, popup and download refusal. None of those service gates is claimed by this native result.

The two initial failed native runs remain in temporary evidence folders `drawing-page-4e5fa2005f8e472a95042422ce3576df` and `drawing-page-668cf75c8b7c4070bb900dcb97e021f0`. They recorded empty asset bodies and mistaken test expectations about querying a closed controller. The corrected fixture observes script refusal after the documented Close call. An intermediate seven-case pass remains in `drawing-page-3c4001d4a10645df9607c77060faa17b`.
