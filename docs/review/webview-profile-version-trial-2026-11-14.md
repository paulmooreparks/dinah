# Browser profile version trial

The main WebView2 profile folder was reused by two real native processes with different injected versions. The hidden recipe changed the HTTP port between starts, so the second origin had neither storage marker in the separate persistent headless browser. Native WebView2 storage continuity remains unproved because this build's loader blocks the documented debugger attachment.

The original trial used branch engine/profile-trial from master e87cdf5328bd6c28863aa429abdab045e873d173. That trial changed no production Go or frontend application source. Its executable additions were the harness, process observer and reproduction script. The later correction below changes the file response policy and extracts the existing startup address selection for testing. Nothing was pushed, installed or merged, and no mouse, keyboard or focus automation was used.

## What ran

The runner built cmd/dinah-desktop twice with desktop,production tags and `-ldflags "-H windowsgui -X main.appVersion=..."`. The final builds also used `-buildvcs=false`. An earlier build displayed revision b9a6157 although this worktree's HEAD was e87cdf53; the final trial records its package directory and base commit explicitly in source.json instead of treating that automatic stamp as provenance.

Both native processes used one isolated DINAH_HOME under the trial output folder. Before the first launch, the fixture created an empty webview2/0.0.0-profiletrial.1 folder to exercise adoption of a legacy version-named profile. It did not prepopulate browser data. The first process chose and recorded that folder. A separate headless Edge profile, also under the output folder, loaded the actual authenticated application page and committed a localStorage value and an IndexedDB transaction. The native process then completed the normal runtime Quit and page-flush handshake. The driver closed its browser before the second launch.

The second native process used the same application home. Its page was loaded in the same separate headless browser profile. The fixture neither forced a port nor copied storage between origins or profiles. It did not revisit the old origin to manufacture continuity.

| Observation | First version | Second version |
| --- | --- | --- |
| Reported version | 0.0.0-profiletrial.1 | 0.0.0-profiletrial.2 |
| Native process | 55348 | 32236 |
| Page origin | http://127.0.0.1:63213 | http://127.0.0.1:63302 |
| profiles.json main folder | 0.0.0-profiletrial.1 | 0.0.0-profiletrial.1 |
| Profile directories | Only 0.0.0-profiletrial.1 | Only 0.0.0-profiletrial.1 |
| Native WebView2 user-data argument | Recorded folder's EBWebView child | Same EBWebView child |
| External localStorage marker | Present | Absent |
| External IndexedDB marker | Present after transaction completion | Database absent |
| Embedded frontend files returning 200 and no-store | 705 of 705 | 705 of 705 |
| JavaScript page errors | 0 | 0 |
| Native process exit code | 0 | 0 |

The process observer read native window visibility and WebView2 process arguments. All five enumerated top-level windows of each native process were hidden at the checkpoints. It sent no window messages and changed no window state. Both process groups exited, including their WebView2 children. A final scoped process check found no WebView2 processes belonging to any of the retained trial folders.

The profile record stayed unchanged and no 0.0.0-profiletrial.2 directory was created. This establishes actual native folder reuse for the main profile in this trial. It does not establish retention of native page data.

## Origin and cache findings

The hidden hook in main.go explicitly selects 127.0.0.1:0. The observed ports differed. LocalStorage and IndexedDB are scoped to the origin, so the missing values at the second origin do not demonstrate that the first origin's data was deleted. Normal launches use the fixed product shell port 47616 in this source. That production-origin behavior was inspected in source and was not exercised, because launching the normal window would violate the hidden-only instruction.

The asset manifest came from Go's EmbedFiles list for the desktop,production build, rather than a hand-selected set of files. The authenticated server returned status 200 with Cache-Control: no-store for all 705 embedded frontend files on each start. Successful observed page and bridge responses also had no-store.

One optional request on each start returned 404 for /product-applets.json without Cache-Control. That path is not an embedded file in this product. The evidence supports the header claim for the existing page files, but does not support a claim that every HTTP response carries no-store. The missing optional resource and its header exception remain in the receipt.

## Native storage boundary

The standard Playwright WebView2 route uses WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS and chromium.connectOverCDP, as described in the [Playwright WebView2 documentation](https://playwright.dev/docs/webview2) and [Microsoft's debugging documentation](https://learn.microsoft.com/en-us/microsoft-edge/webview2/how-to/debug-visual-studio-code). The first attempt used that route and timed out before making a storage observation.

The pinned github.com/wailsapp/go-webview2 v1.0.22 implementation calls preventEnvAndRegistryOverrides in webviewloader/env_create.go and empties the additional-browser-arguments variable. It also prevents registry overrides. Wails' hidden recipe deliberately leaves its native page blank; the external browser is the active application page. Therefore the successful storage observations belong to the separate persistent headless Edge profile. They cannot be presented as native WebView2 storage evidence.

No loader replacement, extra loader build tag, registry change or native script-injection workaround was used. Completing the native data proof needs an approved way to evaluate a page in the hidden native WebView2. The documents and show profiles, fixed-origin persistence and an installed release update were not tested.

## Evidence and reproduction

The completed receipt, source record, manifest, executable hashes and isolated logs are in `%TEMP%/dinah-profile-version-trial-20261114-observed`. The receipt's SHA-256 is a92c59a072f1d55e508a5f9e9ea3ae19fffe42cb19eb0c174ea5c772cf327c89. The manifest's SHA-256 is c6c8d9baa8322dd531771bb3e278199d389baaa117721469021394bda4b608c2.

| Executable | SHA-256 |
| --- | --- |
| 0.0.0-profiletrial.1.exe | a601d875333f7f80fcb3423b94cf7ed175fc66c19392510cd4bbdef4ffb89067 |
| 0.0.0-profiletrial.2.exe | 48e2bbb9a0d7a5daf4e5206ef2ef0d38bcebb1f871485ef63bd26a6d5d658af6 |

The completed reproduction command exited zero under the shared FIFO:

```powershell
scripts/with-test-lock.ps1 pwsh -NoProfile -File scripts/profile-version-trial.ps1 -Out <new-temporary-output-folder>
```

The script requires installed headless Microsoft Edge and installs the pinned Playwright Core 1.64.0 driver into the output folder. It refuses an existing output folder. PROFILE_TRIAL_NATIVE=1 selects the original documented debugger probe, which remains unavailable with this production loader. No full browser suite or additional Go test suite ran; the product package was built twice and the changed harness was syntax-checked and executed.

Earlier evidence remains in the sibling output folders. The initial unsuffixed folder records the blocked native debugger attempt. The headless folder records the overly strict displayed-version assertion. The final folder records the overly strict equality check against WebView2's EBWebView child path. The checked folder records the overly broad assertion that included an optional missing-file response. These attempts were retained, and none is counted as a successful complete trial.

## Missing-file correction and fixed-port guard

Lead accepted the trial's limits and requested two follow-ups before merging this branch. The new missing-file regression first failed for GET and HEAD requests to the optional product-applets.json, an ordinary missing script and a public PUDL stylesheet. Each returned 404 without Cache-Control. The normal-start port regression passed.

The shell now applies no-store when its page-file writer commits a status or body. This keeps Go's file-error handling from discarding the policy. The wrapper is confined to static page files; it does not wrap the bridge or its streaming responses. The main page, public PUDL path and slide show's static files share it. Tests also cover an existing script and an unsatisfiable range response, plus the show's own authenticated missing-file route.

Normal startup now calls shellStartupAddress(false), which returns the profile's ShellAddr. Its regression independently splits the selected address, checks loopback and the configured nonzero ShellPort, and verifies repeated normal selections stay equal. The hidden selection remains 127.0.0.1:0. The test binds no port and opens no window.

The focused correction gate passed before the additional show/range cases were added. All correction source and regressions are committed at 1516c6d5f59b2ec47fe07b3512db180b3a5b6de7. The complete touched-package gate then passed under the shared FIFO:

```powershell
pwsh -NoProfile -File scripts/go-gate.ps1 ./cmd/dinah-desktop/... -count=1
```

The command exited zero. The desktop package passed in 234.912 seconds; its product subpackage has no test files. The original native/storage trial was not rerun, and native WebView2 storage remains unproved. No debugging capability was added to the production loader. This follow-up used Go tests only and did not open a window or use desktop input. Nothing was pushed, merged or installed.
