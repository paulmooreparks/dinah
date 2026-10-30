# Restore and resize profiling

`restore-resize.cjs` uses the installed application's authenticated HTTP bridge in a hidden, isolated test instance. Give it the isolated state directory containing `entry.txt` and copied conversation records. It selects the four largest non-deleted histories, restores them into quarter windows, and compares baseline rendering with experimental message-level content visibility over 24 viewport sizes.

The harness blocks provider opening and resolves each selected saved identity to itself. This prevents competing provider writers and measures saved-history restoration rather than provider reconnect. UI-state writes are blocked. The original state directory must never be supplied as the test directory.

Results contain counts and timing aggregates, without conversation text or session identifiers. Browser authentication state and results remain in the isolated directory and must not be committed. Browser performance metrics distinguish script, style, layout and total task duration. These measurements do not measure native WebView2 composition or establish the latency of provider reconnection.

The containment variant is an experiment. It is not enabled by this harness in production assets.

Set PROFILE=1 to collect aggregate timeline events and sampled self time. VARIANTS can select baseline, offscreen, scan-only or combined. Candidate routes modify only exact application asset paths and retain bridge isolation; the harness asserts that the provider-opening block is present before opening histories. This source-patching harness was measured against installed c45ba6c. A later installed build may already include these changes, so it requires a separate preserved baseline for a valid before/after comparison.

The 120px intrinsic block estimate follows the measured baseline median of roughly 106px. The earlier 200px experimental runs are retained in the design findings; comparisons must identify which candidate they used. No raw performance trace or CPU profile with conversation material is persisted by this harness.

For an isolated library comparison, place a reviewed snapshot at `pudl-menu-candidate.js` in the isolated state directory. Set `MENU_COMPARE=1`, `PROFILE=1`, `RIBBON=1` and `VARIANTS=baseline,menu-only,baseline,menu-only`. Only the exact `/pudl/pudl-menu.js` asset is substituted in candidate runs. Record the candidate hash and installed revision with the results. This mode writes `menu-results.json` and does not change vendored assets. The ribbon workload selects View before restoration and resizing. Native CPU samples include their immediate caller when available, allowing global selector costs to be attributed to their owner.


## Extension integration smoke test

`extension-live.cjs ISOLATED_DIRECTORY SIGNED_PACKAGE` runs against a hidden Dinah instance with `DINAH_HOME` under a disposable directory whose name starts with `dinah-extension-live-`. Set `DINAH_SHELL_ENTRY_FILE` to that directory's `entry.txt`. The test creates its own repositories, installs the signed package into that isolated state, drives actual Git operations through the native host and captures screenshots there. It never accepts the person's production state directory. Its browser authentication file and private fixture data must not be committed. The package may be the signed `dinah-git/dist/extension` folder.

If the isolated directory also contains `dinah.exe`, the smoke test checks CLI disable, enable, confirmed removal and reinstallation against the running host. It uses the signed package argument and verifies that the backend can start again.

## Native fullscreen smoke test

`native-fullscreen.cjs ISOLATED_DIRECTORY` requires an isolated directory named `dinah-fullscreen-live-*`, containing `entry.txt`, `pid.txt` and `native-executable.txt`. Build the fixture with `-tags desktop,production`; a default development build is not a native integration fixture. The executable path must match the recorded test PID before the PowerShell probe touches a window. The probe uses documented Win32 APIs to inspect titlebar style and placement and exercise maximization and minimization. The test toggles real native fullscreen and may briefly show the disposable window; it hides that window on completion. Authentication, screenshots and test state remain outside the repository.

## Native exit smoke test

`native-exit.cjs ISOLATED_DIRECTORY` requires a production-tagged native host with isolated state under `dinah-exit-live-*` and its one-time `entry.txt`. It registers unfinished work, requests native Close, chooses Cancel in the PUDL dialog and verifies that work and the host remain available. A second Close chooses Exit Dinah and verifies that the normal layout flush shuts down the native host. It does not control the person's running application.
