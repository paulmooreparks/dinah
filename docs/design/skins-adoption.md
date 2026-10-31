# Desktop and web skins adoption

Dinah adopts the released PUDL 0.95.2 skin catalogue through the contract in docs/plans/pudl-skins.md. The cross-project architectural source remains C:/Users/paul/OneDrive/Documents/Architectural Principles.md. This candidate combines Engine's reviewed catalogue/asset validation with desktop startup, Settings, preview and contrast integration, plus stylesheet-failure handling in the real web host. Its source base is released c253522, with backend 188ec38ff65afcc0c4a07604330c00746971879d cherry-picked as 6fc741e. The later history release is independent and remains on master.

Static chat-state glyphs and title-marker suppression remain blocked on a published PUDL contract. No PUDL asset, glyph mapping or lead/group chip is changed here. Taskbar workspace-label ordering has a separate record in taskbar-labels.md.

## Preference and first paint

The root selects host persistence. Static admitted skin links load before appearance-init.js and PUDL's theme and skin runtimes. Host styles and palette overrides follow those sheets. Dinah's startup cache, dinah-appearance-startup-v1, contains only the last skin and mode; native reads remain authoritative. The decoded cache must be a non-null object and cannot be an array; other JSON values receive an empty-record fallback. The former browser theme key supplies first-frame migration when that cache lacks a valid mode. The existing native UI-state pudl-theme key stays compatible with the web host.

Settings takes admitted IDs and labels from ReadAppearance. Skin selection persists through SaveAppearance and changes root selection without replacing applets or moving windows. Drafts, focus, text selection and addresses remain intact. Native appearance events and resync reread the preference. Unknown skin preferences stay saved while Standard is effective, and Settings explains the fallback. A failed save or refresh preserves the existing state and draft.

A script before the static links records stylesheet load/error events. Appearance startup unregisters failed sheets before the page body is drawn. Later load/error events use PUDL's public registration API. Skin-change events update availability wording and existing previews. Removed or disabled sheets cannot remain effective. Settings rebuilds its font fields only when their document/override source changes, so a skin-only change retains unsaved font edits.

## Preview and contrast

Preview documents load the admitted sheets, host palette defaults and host font/line-height sheet. Open previews keep their iframe elements and follow the effective skin.

The audit uses a disposable, laid-out document with applicable host styles and candidate typography. It resolves gradient direction, stop positions and repetition at shared coordinates, interpolates premultiplied colors through CSS color-mix, and composites correlated layers. Unsupported paint or positioning syntax fails explicitly. It samples glyph positions using a Canvas mask aligned to the DOM text baseline, excluding pixels below 90 percent coverage. Inputs also receive a boundary check. Resting, hovered, pressed and focus-visible selectors are evaluated through equal-specificity CSSOM instrumentation in this isolated document. Real pointer and keyboard states receive separate screenshot-based verification.

The geometry/interpolation contracts are CSS Images 3 and CSS Color 4 at https://www.w3.org/TR/css-images-3/#linear-gradients and https://www.w3.org/TR/css-color-4/#interpolation. CSSOM interaction instrumentation never changes the application's sheets or PUDL assets.

Each audit owns an immutable copy of its document before calculating a cache key. This prevents an edited preview from changing an in-flight audit and poisoning an earlier cache entry. Cache identity includes the document, catalogue, host-sheet URLs and asset availability. The audit supplies both palette forms and candidate typography itself, so its result is independent of the currently displayed mode. Startup primes the cache; normal skin-only switches reuse it without new asset requests.

The isolated audit checks underlying palettes even when the operating system forces colors. Its positive, fixed-width viewport normalizes forced-color media predicates and disables automatic substitution. The visible application's forced-color behavior remains unchanged. Old custom themes that fail a skin-specific check remain saved and receive an explanation. A rejected operation writes nothing, and the user can explicitly correct the document. Theme edits check all admitted skins; skin-only selection checks the actual effective skin, including Standard fallback.

## Built-in palette correction

Quality reproduced Mineral's original Standard primary contrast at 4.450:1. The first all-stop bounds were conservative and included impossible layer combinations or areas outside text. They were replaced before palette acceptance. The full real-interaction matrix then confirmed 42 failing presentations in 19 theme/mode/control groups across eleven built-in themes.

Only semantic palette inputs were corrected. Light primary/title failures use darker accents, dark pressed-primary failures use darker on-accent text, and confirmed dark destructive/title failures use adjusted semantic foregrounds. Existing category rules still pass. Nord's replacement accent is #405f90, within six OKLab hue degrees of its original blue; a green-shifted diagnostic candidate was rejected. Gruvbox's danger correction retains its required separation from category colors. Mineral's shared desktop/web brand defaults both use #385a74. User-authored themes and PUDL's shape/derived paints are untouched. Nord's derived dark deleted-line tint is now 9 percent, recorded by the existing exact-value regression without changing tint thresholds.

## Verification and remaining boundary

The following evidence is retained under the frontend worktree's .diagnostics directory. Failed and interrupted runs remain available and establish no acceptance.

| Evidence | Result |
| --- | --- |
| skins-focused-20261031 | Original failures, all-stop bounds and screenshots retained |
| skin-rendered-20261031 | 1,144 actual presentations; 42 original failures with paired images |
| skin-palette-fit-20261031-* and skin-nord-hue-20261031 | Rejected and accepted diagnostic candidates, semantic guards and hue constraint retained |
| theme-editor-debug-20261031 | Live debugger captured the custom-editor rejection |
| skin-cache-before-20261031 | Controlled stylesheet delay reproduced mutable-input cache poisoning |
| skins-corrected-20261031 | Stopped owned run after rejecting Nord's hue drift; incomplete |
| skins-corrected-20261031-hue | Corrected palettes passed all 1,144 actual presentations; other checks passed except the editor race and old tint expectation |
| skins-final-focused-20261031 | Seven affected suites passed in 48 seconds, including editor apply/export, cache race, custom-theme preservation and startup fallback |
| skins-final-focused-20261031/go-packages.jsonl | Live-tag vet passed; skins, web and desktop package tests passed; desktop took 105.192 seconds |
| skin-forced-colors-20261031 | Invalid underlying palette remains detectable while the visible application retains CanvasText |
| skins-full-20261031 | 162 of 163 suites passed in 497 seconds; the sole failure was an obsolete taskbar label expectation |
| taskbar-label-corrected-20261101 | Updated workspace-first expectation passed |
| web-skins-20261101 | Real web-host cases passed except the reproduced false-effective-skin result after a stylesheet 404 |
| skins-both-hosts-20261101 | Five affected suites passed in 40 seconds after the shared asset tracker correction |
| skins-both-hosts-20261101/go-packages.jsonl | Live-tag vet and affected skins, web, fixture and desktop package checks passed; desktop took 127.222 seconds |
| skins-final-full-20261101 | The immutable candidate's final full-gate outcome is recorded in candidate-review.md beside this output |

Startup coverage includes every admitted skin in light/dark, unknown preferences and failed stylesheets. Switching covers both motion preferences, layout, mounted nodes, drafts, focus, selection, URLs, failed persistence, native events and asset availability. Screenshots cover wide/narrow workbench, chat, group, Settings and theme-editor surfaces. Representative images were inspected; Quality still owns independent rendered acceptance of the immutable result.

The web browser suite builds internal/web/skinfixture and uses the actual embedded templates and assets through web.New. Its helper process owns a temporary home and isolated bus, signs in through the real cookie flow, and checks desktop, inbox, groups and a seeded conversation at wide/narrow widths under both motion preferences and light/dark. Signed-out, unknown-preference and failed-stylesheet cases are included. It never connects to the running application. A shared early asset tracker prevents a failed stylesheet from remaining effective; the web page explains Standard fallback while preserving the saved choice. The helper requires Go during this browser suite.

Release retains merged-master full gates and installation. No app restart, installation or push is performed by Front End. Existing history growth, typing/crash reports, external saved-history viewing and earlier unexplained gate diagnostics remain open and outside this correction.

Quality found that JSON null bypassed the parse-error fallback in 980bd6d and threw before startup attributes were assigned. The successor normalizes non-record cache values and adds a delayed-native regression for legacy dark fallback, later native Gel/light adoption and absence of page errors. The browser regression is queued behind Quality's current shared-slot review; no earlier full-gate result is attributed to this successor.

The first merged release gate stopped on the fixture's hard-coded provider prefix. The correction derives the fixture session ID from the current profile's default provider row and its documented SessionPrefix. It does not change the enforcement test or its allowlist. The original merged-gate failure remains preserved by Release; corrected profile and actual web-host checks are recorded in .diagnostics/skins-provider-fixture-20261101.
