# Wails 2.13 verification

Wails is upgraded from v2.11.0 to v2.13.0. `go get` and `go mod tidy` ran with Go 1.26.10. The branch was then based on the separately delivered Go 1.26.11 commit `540ae828` to resolve a generated-notices merge conflict before receiving. Its final Go requirement is inherited from that base, and the notices were regenerated with Go 1.26.11. The Wails commit changes only the framework dependencies, their notices, a startup-test expectation and this record. No product UI code was changed.

The Windows amd64 run of `scripts/vulnerability-gate.ps1` with Go 1.26.10 no longer reports GO-2026-6862. It still rejects the desktop target for GO-2026-6859 in the old standard library. That remaining finding is addressed on the separate Go 1.26.11 branch. Reports remain in `%TEMP%/dinah-wails213-vuln`; the nonzero gate result is retained rather than reported as a clean scan.

The desktop, command program and isolated ACP stand-in were built with Go 1.26.10. The real hidden-desktop journey passed all 17 checks through real App methods and the bus helper. It exercised numbered requests, a partial answer, the completed answer, plain-mention navigation and acknowledgement, and a confirmed broadcast to a working chat. Its disposable home, program log and passing `receipt.json` remain in `%TEMP%/dinah-wails213-hidden`. This check drives the real host's page in headless Edge; it does not inspect native WebView2 pixels or use a real model.

The pre-rebase product binaries were rebuilt after regenerating the embedded notices. Their hidden-desktop run also passed all 17 checks, with no page errors. Its receipt and isolated state remain in `%TEMP%/dinah-wails213-hidden-final`. That desktop binary identifies Go 1.26.10 and Wails v2.13.0.

The 17 window-related browser suites ran headlessly in 45 seconds. Sixteen passed. `startup-stable` failed its requirement to observe default Notifications windows being constructed before the saved layout. The recorded frames instead show direct saved-layout construction. This assertion concerns an intermediate implementation choice; all visible-layout stability assertions passed. The original receipt remains in `%TEMP%/dinah-wails213-window-suites`.

An untouched-master run passed that suite in eleven seconds, demonstrating that the intermediate frame is not reliably observed. The test expectation is corrected here and on the separate CI-repair branch. It now requires a recorded unfinished layout behind the covering screen while preserving every visible-layout assertion. The baseline receipt remains in `%TEMP%/dinah-startup-stable-baseline`.

All 17 window suites then passed in 37 seconds. Their final logs and screenshots remain in `%TEMP%/dinah-wails213-window-final`. The startup-test correction changes no product code and is identical on the CI-repair branch.

The first full Go gate passed every package except the stale generated-notices check. Regenerating the notices corrected that focused check. Its original log remains in `%TEMP%/dinah-wails213-gate.log`.

The fresh `pwsh scripts/go-gate.ps1 ./... -count=1` run passed with Go 1.26.10 and exit zero. Its complete log remains in `%TEMP%/dinah-wails213-final-gate.log`.

The initial Go 1.26.10 candidate is retained at `f107c375` on `quality/wails-2.13-go12610-evidence`. Verification of the combined source with Go 1.26.11 is recorded separately so that the two compiler versions are not conflated.

Consumers that vendor Go dependencies must refresh their vendor directory after merging this upgrade. No branch was pushed and no product was installed during verification.
