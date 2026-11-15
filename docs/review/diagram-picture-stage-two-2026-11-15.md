# Diagram picture service stage two

The implementation candidate is `de432c4e8c7fc97ce6d3973b4afe18f5c09bf830` on `engine/drawing-page`, stacking the service on native commit `bc71ad23`. The design owner's startup decision is recorded in contract commit `e6450a1f` and in this branch's `docs/design/diagram-picture-helper.md`. The first native stage was independently accepted against `bc71ad23`; this record covers the service added afterwards.

## Implementation

The window owns the service on its existing loopback listener. Its drawing routes precede page-cookie authentication and require their own workspace-bound bearer lease. An ordinary applet context, page cookie or unissued string cannot authorize them. Browser Origin and fetch headers are refused, as is an unexpected Host. The routes offer no CORS access.

Lease issuance is an unexported launcher operation, `issueDiagramLease(chatID, serverName)`. It checks the profile's new `ToolServer.DiagramPictures` permission and the chat's registered workspace. It returns the three documented environment values and a revocation function. No App method or agent catalog offers issuance. The service stores only a hash of each bearer token. Leases expire after fifteen minutes; removal of a workspace or window shutdown revokes them and cancels active work.

The host opens snapshot files through [os.Root's documented confinement](https://pkg.go.dev/os#Root) and applies the existing file-access policy to the final opened handle before reading bytes. The installed Go 1.26.10 API comments specify confinement through symbolic links and refusal of Windows device names. The added `ReadOpenedBytes` operation preserves that identity check and audit while allowing the contract's two-MiB diagram source. The page receives retained source and dependency bytes. It cannot reopen workspace paths. Dependency images must match their declared type and decoded dimension bounds, and Canvas references must resolve to a declared path without basename fallback.

`internal/diagrampicture` implements the frozen client DTOs and constructor. The client makes an authenticated, one-second OPTIONS probe before POST, uses the shared helper-protocol negotiation, refuses redirects and proxies, and validates every repeated snapshot identity, PNG hash, dimension and byte bound. Cancellation closes the POST transport without waiting beyond the caller's deadline for another request.

Admission limits active drawing to one request per workspace and two per window. Request IDs are scoped to a lease; active and recent terminal IDs refuse replay. Terminal records expire after two minutes, and 256 retained terminal records require a fresh lease. Any picture problem prevents bytes being returned. An injected Close failure disables subsequent drawing.

The native page now loads product fonts explicitly, receives verified image bytes and refuses navigation, new windows and downloads through the documented WebView2 interfaces. The small COM bindings use Microsoft's SDK 1.0.902.49 declarations. The SDK archive was read in a temporary folder for those declarations; no SDK or dependency was installed.

## Startup disposal

An unresolved startup at cancellation or deadline returns a typed `drawingpage.StartupError`. Its `Settled` channel preserves ownership of the COM apartment and callback state. The host maps it through `diagrampicture.RecoverAfterStartup`, which closes window-wide admission until late disposal succeeds. A late failed Close makes the refusal permanent for that window. A handler that never arrives leaves admission closed until restart. Private browser data is retained until disposal and is not removed while startup remains unresolved.

The test-only bindings are private package variables in `internal/drawingpage`: `closeNativeController` and `delayStartupCompletion`. Same-package `_test.go` fixtures can replace them; production has no route, argument or environment switch that selects either. The forced Close fixture performs real cleanup before injecting its failed result, so its own run leaves no stuck page behind.

`TestNativeUnresolvedStartup` withholds controller completion while retaining its native reference, cancels the request after the held event, and observes bounded refusal. It then releases the late handler, observes native closure and draws in a fresh page. Its withheld-handler case checks continued unresolved state before releasing the handler solely for fixture cleanup. `TestServiceUnresolvedStartupBlocksAndRecovers` verifies the corresponding HTTP admission rule and permanent refusal after a late Close failure.

## Verification

The ordinary service tests cover authority, equal and absent protocol offers, newer-only and malformed offers, snapshot conflict, replay, per-workspace and per-window capacity, lease-scoped cancellation, workspace and window revocation, retained bytes after mutation, expiry and replay-budget exhaustion. The host test covers profile opt-in, registered workspace binding, the final-handle file-access policy, page-cookie refusal, browser-origin refusal, Host refusal and revocation.

The corrected independent client fixture from Quality commit `ed2ebba5` was copied unchanged for the developer run. Its SHA-256 is `9df8f255a7cc9046c88b3a65b3cf47b405fd5bcd1f9c428133783257d5855e3b`. It remains owned by Quality and is excluded from this candidate's commits. Its verification cases passed in 3.130 seconds together with the service tests then present. This is a developer run of those assertions; Quality's independent execution remains pending.

The first run preserved a fixture hang in Server.Close. Quality corrected its deadline peer to consume the POST body and guarantee fixture cleanup. The corrected fixture then caught the client's extra cancellation request delaying deadline return; removing that wait fixed the product defect. The first startup-recovery run also caught an unconditional permanent-disable assignment after a recoverable refusal. That assignment was removed. The final checks below must pass before handover.

| Check | Result |
| --- | --- |
| Full desktop, file-access, helper protocol and ordinary service packages | Ticket 2000 passed. Desktop took 190.830 seconds and file-access took 49.459 seconds. The final full desktop repeat at candidate source passed through ticket 2006 in 210.508 seconds. |
| Native fixtures and corrected independent client assertions | Ticket 2001 passed in 12.543 seconds for native drawing and 3.187 seconds for the service/client package. The final repeat at candidate source passed through ticket 2009 in 27.021 and 3.069 seconds respectively. Its evidence folder is `%TEMP%\drawing-stage2-candidate-35fdaf998d5c4e0fb087c527f63632f7`. |
| Independent route and additional native bindings | Pending Quality's candidate run. |

## Independent binding surface

The service constructor is `diagrampicture.NewService(host string, renderer Renderer) *Service`. The host value is the actual loopback listener's Host. `Issue(workspace, program string, read ReadSnapshot) (string, error)` is a host-only capability constructor. The injected reader has signature `func(context.Context, string, int) ([]byte, error)` and must open through the registered workspace authority and obey the supplied byte bound. The actual host adapter is in `cmd/dinah-desktop/diagram_picture_service.go`; its authority test uses the real shell listener and policy service.

`Renderer` is `func(context.Context, Drawing) (Picture, error)`. `Drawing` contains `Request`, `Kind`, retained `Source []byte` and `Images map[string][]byte`, keyed by declared workspace-relative paths. The production renderer uses `drawingpage.Draw`. A renderer must dispose its controller before returning, except for the documented typed unresolved-startup result. The service fills and validates the response identity from its verified request. `Revoke`, `RevokeWorkspace` and `Close` cancel admitted work and wait for its terminal return.

Quality's frozen route tests accept an in-process fixture through `QUALITY_DRAW_ENDPOINT`, `QUALITY_DRAW_ROOT`, `QUALITY_DRAW_LEASE`, `QUALITY_DRAW_EXPIRED_LEASE`, `QUALITY_DRAW_APPLET_CONTEXT` and `QUALITY_DRAW_WORKSPACE`. Fixture binding must keep those private to its test process and must not write or log the bearer lease. The revoked-lease value can be issued and immediately revoked. The ordinary applet value is never admitted by this service. No product test route is needed.

The native repeat command remains the one in the first-stage record. With Quality's corrected client file present, the combined command is `go test -tags native_drawing,quality_diagram_picture ./internal/drawingpage ./internal/diagrampicture -count=1 -timeout=90s`, through `scripts/with-test-lock.ps1`, with `DRAWING_PAGE_EVIDENCE` pointing at a fresh temporary folder. The developer copy of Quality's client file was removed after that run; the source remains unchanged on Quality's branch.

## Gates carried forward

The remaining independent gates include the frozen route assertions, final-path and image boundary fixtures, mutation, replay and capacity, cancellation and disconnect disposal, forced Close failure, and both unresolved-startup cases. Quality must rerun the native bindings against this stage because its fonts, resource restrictions and snapshot input have changed.

Provider delivery is the next stage and has not started. Its gates remain custom-registration preservation, the distinct restricted catalog, lease redaction from logs and saved configuration, and documented delivery through Codex, Claude and Ollama. Unsupported delivery must refuse visibly. No product profile enables drawing in this branch yet.

The product adapter's publication gates remain with its owner. They include no upload after any refusal, source and dependency mutation between drawing and upload, publication of retained bytes, existing approval and authorization checks, and unchanged D2 and verified-picture reuse when no lease exists. This service result cannot establish those upload or provider gates.
