# Today's surfaces across skins

The opt-in full visual check uses every admitted entry in the bundled pudl-skins.json catalogue. Standard counts as one skin, alongside Bevel, Gel, Glaze, Matte, Ink, Alloy, Alloy Bold and Glass. Each state is checked in light and dark mode at 1440 by 1000 and 390 by 844 pixels. The build's default colour palette remains selected. This check does not enumerate every separately selectable colour theme.

Both entry points use the same state builders and assertion helper. The default today-themes suite samples a Decisions request with context, broadcast confirmation, the notification stack and Set-aside Programs in Standard and Glass, in both modes and at both widths. The full audit mounts the existing Decisions, Updates, notification, answer-card and stand-alone applets with invented service data. The stand-alone page uses its actual host and bundled Docs, with an invented page bridge. This visual suite performs no native operation or external service call. The earlier hidden-program check remains separate evidence for the stand-alone host.

The suite captures text and then the same geometry with text fill hidden. It identifies solid glyph samples against the actual background pixels, including gradients. Ordinary text requires 4.5:1 contrast, while large text requires 3:1. It records text for which no solid sample was found and excludes disabled controls. Each viewport slice must contain measured text. A focused control must show focus-visible and at least ten edge pixels with a 3:1 change from its unfocused paint. The suite also measures button clipping and control overlap, checks the new status masks and colours, and saves a screenshot of each state and combination. Its measurements describe those sampled states and controls rather than a complete accessibility audit.

The pilot exposed the translucent focus glow on today's controls. The new styles add a two-pixel solid accent outline with an offset; outlines reserve no layout space. The styles use PUDL's accent, danger and spacing tokens. Existing status lines gain an information mask, and failed status lines gain the warning mask and danger token. Decisions' structured rejection state selects that warning treatment, and a failed read-only lookup does the same. The rendered words and operations stay with their existing owners.

The architectural principles remain at C:\Users\paul\OneDrive\Documents\Architectural Principles.md.

## Running the checks

The ordinary browser run discovers today-themes.js and runs its 32 state combinations. It never discovers today-themes-full.cjs because run.ps1 enumerates only .js entry points. Both entry points call the same contrast, focus, status and geometry assertions; the default changes only the states and skins selected.

Run the default check from the repository root with the browser dependencies available:

```powershell
./cmd/dinah-desktop/frontend/e2e/run.ps1 -Only today-themes -Jobs 1 -Out "$env:TEMP/today-themes-fast"
```

Run the complete nine-skin audit explicitly through the shared test queue:

```powershell
./scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/today-themes-full.cjs http://dinah.test/index.html "$env:TEMP/today-themes-full"
```

Leave DINAH_TODAY_SURFACE unset for the complete audit. For a focused opt-in run, set it to decisions, notification, settings, answer, standalone or extras, then clear it afterwards. This filter changes only which state builder runs. The full entry point still uses all nine skins in both modes at both widths. The earlier pilot-only skin variable has been removed.
