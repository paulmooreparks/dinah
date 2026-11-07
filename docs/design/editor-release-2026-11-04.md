# Editor and PUDL integration, 4 November 2026

This candidate combines the Editor status bar change with unmodified PUDL 0.97.10. It starts from Dinah baseline `3b9fe2c22a9c71f4e3fe28722e86f7af19e62e1e`, preserving the installed performance candidate's Go and JavaScript outside the Editor changes. The rejected shared-geometry prototype, parked Save changes, and telemetry proposals are excluded.

Paul also requested live full-window resizing after reporting no sluggishness during this round. The desktop layer now explicitly selects `data-win-resize-mode="live"`, as documented by PUDL's CONTRACT.md. The existing workbench test now checks that mounted content changes size during pointer movement, before release, without an outline preview. Paul's observation is useful live feedback but is not a controlled performance comparison.

Both `cmd/dinah-desktop/frontend/pudl` and `internal/web/static/pudl` contain all 183 distribution files from the release archive. Every file was hash-checked against the extracted archive. Their VERSION files record 0.97.10. The former local selector patch is now supplied by PUDL itself, including the minified distribution.

The archive is `pudl-0.97.10.zip`, with SHA256 `c87f04a61d496e0ba01e8be6231a0fd3009c2c2cac7a1f743aab45de0bfdc8d8`. The published checksum matches the downloaded bytes. The upstream immutable commit is `867a3d038f3b3bae02555ac14622060a21c460c9`.

PUDL's release retains the measured selector improvement and the skin-observer correction. Its evidence does not establish that intermittent live stalls are resolved. The upstream lead reports independent review and 54 passing current-version browser suite runs. GitHub's Tests workflows were still running when integration began.

## Verification and delivery

The integration gate covers the Editor, embedded skill editors, skins and appearance, server-rendered skin pages, chat windows and restoration, scroll position, taskbar stacks, window switching, menus during updates, and diagrams. Browser tests use explicit headless Edge and page protocol input. Go checks cover the desktop and web packages. The candidate will be built only after those results are assessed.

All 15 integration suites passed in 199 seconds with one suite running at a time. The empty failure report and screenshots are in `build/pudl-0.97.10-integration`. Live resizing has a separate focused workbench check because Paul requested that change during the integration run.

The workbench suite passed in 9 seconds, including content resizing before pointer release. The desktop Go package passed in 122.591 seconds and the web package passed in 9.015 seconds. The web skin fixture has no Go tests and was exercised by the browser suite. The Windows executable built successfully with Go 1.26.10 as `0.0.0-editor-pudl-20261104`. Exact embedded bytes were checked for the Editor files, index, window stylesheet, and skin script.

Installation, restart, and native desktop input are not part of this integration step. Native clipboard behavior and native host presentation have not been newly exercised. The existing performance measurements remain historical evidence for the selector patch; this integration does not repeat the performance benchmark.

## Far-right chip update

Paul authorized building and installing the chip placement adjustment on 4 November 2026. The focused Editor suite passed in 42 seconds, and wide and narrow screenshots were inspected. The build uses version `0.0.0-editor-chip-20261104` and retains the preceding Editor, PUDL, and live-resize changes. The compiled executable contains the exact current Editor JavaScript, stylesheet, and index bytes.

Installation completed at 18:54:14 Singapore time. The installed executable is `C:/Users/paul/.dinah/bin/dinah-desktop.exe`, with SHA256 `766c76386e462aa78b66f30b8cab7c6c829c3e8799c35fa6024cfb733888dd1f`. The build remains in `build/editor-chip-20261104/dinah-desktop.exe`. The previous executable was preserved as `C:/Users/paul/.dinah/bin/dinah-desktop.exe.old-chip-20261104-185414`, with SHA256 `460ba002b1a7243f4b0fb2f4b4ba40038ef3db43d10e705755ac5ba8829d9a3f`. Staged, installed, and rollback hashes were verified. The running desktop was not restarted.
