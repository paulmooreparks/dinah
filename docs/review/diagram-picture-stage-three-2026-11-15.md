# Diagram picture provider delivery

The source candidate is `4b302550`, following initial provider commit `4734af82`, on `engine/drawing-page` in `.worktrees/engine-drawing-page`. It stacks provider delivery and the native pointer correction on stage two `80064e68`. No product profile enables the capability in this branch. The generic branch contains no product-specific restricted command or catalog names.

## Registration and authority

A profile opts in with `ToolServer.DiagramPictures`, supplies `DiagramPicturePrefix` and `DiagramPictureArgs`, and keeps the existing `Program` as the installed executable name. The prefix accepts an ASCII letter followed by up to 39 letters, digits, underscores or hyphens. The host adds ten random base32 characters. It copies the restricted arguments without changing them. Missing restricted configuration or an installed program refuses activation rather than starting another full catalog.

The four environment values are `DIAGRAM_PICTURE_ENDPOINT`, `DIAGRAM_PICTURE_LEASE`, `DIAGRAM_PICTURE_WORKSPACE` and `DIAGRAM_PICTURE_ROOT`. The last value is the absolute root from the window's registered workspace and grants nothing. The snapshot reader retains the window's own scope and final-handle policy. Configuration may be delivered before session allocation, but that reader refuses access until the channel is bound to the resulting chat identity. Opening or resuming creates a fresh registration. Channel replacement, session closure and provider loss revoke the previous lease. A fifteen-minute expiry requires reopening the chat. The context explains that boundary without revealing the lease.

The shell's `ConfigureServers` lifecycle hook runs after default managed-server construction for new and reopened sessions. It retains the capability on the applet channel so failure rollback and ordinary lifecycle revocation release it. Codex translates the stdio environment into the documented runtime configuration and retains its effective-configuration conflict check. Claude uses its ACP adapter's documented session MCP list and environment entries. Ollama passes the entries to the host-owned child process. Devin gets no drawing registration and receives the supported-chat refusal in its context.

The design owner's random-alias decision replaces pre-inspection for Claude, whose documented interface offers no effective configured-name listing. Existing registrations are left untouched. Reported conflicts remain errors. The adapter must describe the restricted publication server during MCP initialization and keep each operation discoverable through its own name and description. It owns the catalog and its existing publication approval checks; the lease adds no publication authority.

Channel credentials are registered for trace redaction before protocol serialization. ACP refusals now also redact registered credentials before returning to callers, since a provider may quote rejected environment values in its error. Error codes remain intact.

## Native pointer correction

The earlier stream wrapper converted a native `uintptr` return value to `unsafe.Pointer` for Release. Broad vet rejected that conversion. Native COM addresses now remain `uintptr` through dispatch. The binding uses documented [ReadProcessMemory](https://learn.microsoft.com/windows/win32/api/memoryapi/nf-memoryapi-readprocessmemory) on the current process to copy the vtable and method addresses into Go values, then passes the address unchanged to `SyscallN`. This avoids relying on a stored integer-to-pointer conversion. The test-only Close and delayed-startup seams keep their signatures.

## Developer verification

The focused host and provider checks passed, including all four environment values reaching child processes through Codex and Claude protocol stand-ins and the real Ollama MCP process launcher. These stand-ins exercise transport and launch configuration, without an account-backed provider. They do not claim a live Codex or Claude service run. The host checks preserve a custom ordinary entry, pass restricted arguments unchanged, use different aliases on reopen, deny an unbound reader and revoke the replaced lease. Devin receives a refusal and no extra server.

The Windows confinement check opens an ordinary source successfully, then refuses an outside file symlink, an outside directory junction and a relative in-workspace symlink to a policy-denied source. It uses the real host reader and file-access policy with an invented renderer. The refused paths never enter that renderer.

The first developer fixture runs had two harness errors. A confinement helper was referenced before its body was finished, and then its endpoint included the route where the client requires a base URL. The Codex stand-in also omitted the existing `thread/inject_items` response and timed out. Those failures were preserved in test output; the completed fixtures pass without changing their expected authority or transport outcomes.

The complete provider, session-host, client, native-package and desktop test batch passed at initial provider source `4734af82`. Package times were 5.829 seconds for ACP, 4.499 for Codex, 0.901 for Claude, 52.123 for Ollama, 8.128 for the session host, 1.154 for the service/client, 0.422 for the ordinary native package and 299.230 for the full desktop package. The native tagged run passed in 13.475 seconds after the pointer correction, including Canvas, Mermaid, animation-frame suppression, stuck-script cancellation/deadline, fresh drawing, verified images, the response-body reproduction, Close failure and delayed startup. Its evidence is `%TEMP%/drawing-stage3-eaee15c59d3a4fa9b34557632dde7525`. Broad `go vet ./...` passed without diagnostics at the final source. The final full desktop package passed in 170.130 seconds at `4b302550`, including the pending-channel revocation test. A review of shutdown ordering found that configuration could race with pending-channel revocation. The follow-up source commit attaches under the bridge lock and immediately revokes any lease issued after the channel has closed. Its test forces both orderings. The final desktop package run covers that follow-up.

## Bindings for Quality's final check

Quality's stage-two acceptance is bounded by its report at `17fa185c`. The remaining gates stay required before a product enables drawing. The following bindings provide those checks without product test routes.

| Gate | Fixture binding |
| --- | --- |
| Links and junctions | Run `TestDiagramLeaseLinkAndJunctionConfinement` in the desktop package. It supplies real isolated reparse points, a profile-issued lease, the host reader and a renderer admission counter. Add dependency paths to that same request and reader for dependency confinement. |
| Dependency changes during native drawing | In a same-package host fixture, issue the lease through `issueDiagramLease`, wrap the service renderer, and mutate the source/dependency paths after it receives `diagrampicture.Drawing` but before calling `drawingpage.Draw`. Pass only `Drawing.Source` and `Drawing.Images`. The wrapper is an in-process test binding. Decode output pixels against the retained image; no product switch is needed. |
| External request observation | Start an isolated counting HTTP peer and use its URL in refused diagram dependencies. The native `Options.Event` records assets, refused resources and ordered closure. The same-package native `draw` fixture may run a fixed hostile resource script to test interception directly. Assert that the peer receives no request and that failed drawings return no PNG. |
| Dependency formats and aggregate limits | Use `NewService` with a bounded `ReadSnapshot` fixture, real encoded PNG, JPEG and WebP dependencies, and a renderer admission counter. `Request.Pictures` and its hashes exercise per-image and aggregate byte limits before native entry. Use the same accepted bytes with the native `Request.Images` map for rendered format checks. |
| Providers and catalog | Run `TestDiagramManagedRegistration` and each provider package's `TestDiagramSessionEnvironmentDelivery`. The former exercises opt-in, the four values, restricted arguments, context and revocation. Codex's stand-in also refuses an effective-name conflict. The product adapter separately owns exact catalog membership, stable operation descriptions and unchanged write/approval metadata. |
| Upload consequences | Combine this candidate's host-issued environment with the product adapter's fake publication server. Its source and dependency mutation fixtures must compare uploaded bytes to the retained snapshots. Refusal, timeout, cancellation, malformed results and any listed problem must leave both attachment and page write counters at zero. The adapter owner supplies the restricted profile and command. |
| Native failure and recovery | Retain Quality's native Close-failure and delayed-controller bindings unchanged. `closeNativeController` and `delayStartupCompletion` are still same-package test seams, with no production routes or flags. |

The native command requires a fresh `DRAWING_PAGE_EVIDENCE` folder and `go test -tags native_drawing ./internal/drawingpage`. Quality can layer its `quality_diagram_picture` tag and frozen same-package fixture bindings onto this source. All tests and builds use `scripts/with-test-lock.ps1`.

No real upload, account-backed provider request, installation, push or merge occurred in this stage.

Quality should run the remaining final bindings against this immutable source and the product adapter before any profile enables drawing. The lead owns the integration and release decision. Engine retains ownership of source findings from those runs.
