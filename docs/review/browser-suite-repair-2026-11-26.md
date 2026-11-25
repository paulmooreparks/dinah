# Browser suite repair

The repair changes only browser tests. The application and its visible interface are unchanged.

## Causes and changes

The tool-retention suite sampled each 250-update batch after a fixed 500 ms. The application's event queue applies updates within a frame budget, so that delay could expire before the last update reached the transcript. The baseline samples contained 155, 369, 518 and 561 tools after sending 250, 500, 750 and 1,000 updates. The latest-tool assertion consequently ran before its tool arrived. The suite now waits for the final tool of each batch to appear in the current chat before taking its sample. The 600-row bound and latest-tool assertions remain unchanged.

Both skin suites exceeded the runner's 600-second limit while their completed contrast and interaction-state assertions passed. Each matrix entry took two locator screenshots, repeating locator resolution, scrolling and stability checks for a fixed probe whose animations are disabled. The harness now captures the probe's viewport rectangle through the page screenshot API. It refuses a missing or clipped rectangle and checks that hiding the label leaves the rectangle unchanged. Hover and press move the real pointer to the control's measured centre, and the existing CSS-state assertion still verifies the result. The pixel comparison still measures the visible glyph against the same control with its label hidden. The full theme, mode, skin, control and state matrix, glyph sampling rules, 4.5 contrast threshold and suite timeout remain unchanged.

## Verification

The baseline at `42d83f537bd1c41bbb727ec1eb162bb822e04317` reproduced all three failures with the normal headless runner. The tool-retention suite failed its latest-tool assertion; both skin suites timed out. The run took 600 seconds. Its full logs and available artifacts remain in `%TEMP%/quality-browser-repair-before`.

The first repair run passed all three suites in 596 seconds, including 1,872 and 1,638 distinct skin measurements with no contrast failures or page errors. Its logs, contrast records and screenshots remain in `%TEMP%/quality-browser-repair-after`. The four-second timeout margin prompted removal of redundant frame waits and locator-hover work before the final skin run.

The final skin run passed both suites in 444 seconds, leaving 156 seconds under the unchanged limit. Its records contain the same 1,872 and 1,638 measurement identities as the first passing run, with zero contrast failures, interaction-state failures or page errors. The complete matrix covers 15 themes and nine skins. The final logs, records and screenshots remain in `%TEMP%/quality-browser-repair-final`. The tool-retention file is unchanged since its passing run.

```powershell
pwsh ./run.ps1 -Only chat-tool-retention,skin-paints,skin-paints-2 -Out <evidence-folder>
```

The command runs from `cmd/dinah-desktop/frontend/e2e`. No other browser suites, builds, installs or desktop interactions are part of this repair.
