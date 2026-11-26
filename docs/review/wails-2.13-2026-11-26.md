# Wails 2.13 verification

Wails is upgraded from v2.11.0 to v2.13.0. `go get` and `go mod tidy` ran with Go 1.26.10. The branch was then based on the separately delivered Go 1.26.11 commit `540ae828` to resolve a generated-notices merge conflict before receiving. Its final Go requirement is inherited from that base, and the notices were regenerated with Go 1.26.11. The Wails commit changes only the framework dependencies, their notices, startup and round-count test expectations and this record. No product UI code was changed.

The Windows amd64 run of `scripts/vulnerability-gate.ps1` with Go 1.26.10 no longer reports GO-2026-6862. It still rejects the desktop target for GO-2026-6859 in the old standard library. That remaining finding is addressed on the separate Go 1.26.11 branch. Reports remain in `%TEMP%/dinah-wails213-vuln`; the nonzero gate result is retained rather than reported as a clean scan.

The desktop, command program and isolated ACP stand-in were built with Go 1.26.10. The real hidden-desktop journey passed all 17 checks through real App methods and the bus helper. It exercised numbered requests, a partial answer, the completed answer, plain-mention navigation and acknowledgement, and a confirmed broadcast to a working chat. Its disposable home, program log and passing `receipt.json` remain in `%TEMP%/dinah-wails213-hidden`. This check drives the real host's page in headless Edge; it does not inspect native WebView2 pixels or use a real model.

The pre-rebase product binaries were rebuilt after regenerating the embedded notices. Their hidden-desktop run also passed all 17 checks, with no page errors. Its receipt and isolated state remain in `%TEMP%/dinah-wails213-hidden-final`. That desktop binary identifies Go 1.26.10 and Wails v2.13.0.

The 17 window-related browser suites ran headlessly in 45 seconds. Sixteen passed. `startup-stable` failed its requirement to observe default Notifications windows being constructed before the saved layout. The recorded frames instead show direct saved-layout construction. This assertion concerns an intermediate implementation choice; all visible-layout stability assertions passed. The original receipt remains in `%TEMP%/dinah-wails213-window-suites`.

An untouched-master run passed that suite in eleven seconds, demonstrating that the intermediate frame is not reliably observed. The test expectation is corrected here and on the separate CI-repair branch. It now requires a recorded unfinished layout behind the covering screen while preserving every visible-layout assertion. The baseline receipt remains in `%TEMP%/dinah-startup-stable-baseline`.

All 17 window suites then passed in 37 seconds. Their final logs and screenshots remain in `%TEMP%/dinah-wails213-window-final`. The startup-test correction changes no product code and is identical on the CI-repair branch.

The first full Go gate passed every package except the stale generated-notices check. Regenerating the notices corrected that focused check. Its original log remains in `%TEMP%/dinah-wails213-gate.log`.

The fresh `pwsh scripts/go-gate.ps1 ./... -count=1` run passed with Go 1.26.10 and exit zero. Its complete log remains in `%TEMP%/dinah-wails213-final-gate.log`.

The initial Go 1.26.10 candidate is retained at `f107c375` on `quality/wails-2.13-go12610-evidence`. Verification of the combined source with Go 1.26.11 is recorded separately so that the two compiler versions are not conflated.

The combined-source `scripts/vulnerability-gate.ps1` runs with Go 1.26.11 passed both executables on Windows amd64 and arm64, with no called vulnerabilities. The reports remain in `%TEMP%/dinah-wails213-go12611-vuln-amd64` and `%TEMP%/dinah-wails213-go12611-vuln-arm64`.

Consumers that vendor Go dependencies must refresh their vendor directory after merging this upgrade. No branch was pushed and no product was installed during verification.

With Go 1.26.11, rebuilt product binaries passed the hidden real-host journey again (17 checks, no failures or page errors), and all 17 window suites passed in 37 seconds. The respective receipts remain in `%TEMP%/dinah-wails213-go12611-hidden` and `%TEMP%/dinah-wails213-go12611-windows`.

The first combined full Go gate failed only the Ollama round-count fixture's unrelated 30-second deadline, which cancelled after 75 requests. The fixture now uses the test lifetime context without reducing its tool rounds. Its resume assertion requires all 101 requests and normal completion. The whole Ollama package passes in 54.527 seconds with Go 1.26.11. The failed receipt remains in `%TEMP%/dinah-wails213-go12611-gate.log`, and the focused passing receipt remains in `%TEMP%/dinah-ollama-rounds-fixed.log`. This test-only correction is identical on the separate CI branch.

The final combined full Go gate passed with Go 1.26.11 and exit zero after the round-count fixture correction. Its complete package results remain in `%TEMP%/dinah-wails213-go12611-final-gate.log`. This gate covers the final product and test sources; no failure was hidden or retried without a correction.
