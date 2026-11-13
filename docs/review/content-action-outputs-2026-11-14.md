# Content action output review

Quality investigated on branch `quality/diagrams-new-output`, based on master `bd6f823f`. The package is yavc.app v0.2.88. Only Dinah host source and focused tests changed. Packaged files, the output schemas and the broker validator remain unchanged.

## Cause and correction

The original agent New failure was already corrected on master by `95c4aaa0`. Its list-row response lacked `drawn` and `problems`. The remaining `documentWindows.open` route had the same omission. It already returned an `output` wrapper; the suspected missing wrapper was not the defect in this source.

The routed baseline response for a Canvas New request was:

```json
{"output":{"index":1,"name":"Untitled.d2","path":"Untitled (not saved)","kind":"canvas","modified":false,"view":"split"}}
```

The packaged `yavc.diagrams/new@1` output schema requires `index`, `name`, `path`, `kind`, `modified`, `view`, `drawn` and `problems`. The first is an integer, name and view are strings, path is a string or null, kind is d2/mermaid/canvas, modified and drawn are booleans, and problems is an array of objects with a string message and integer line. Dinah's Go validator rejects the response above with `missing properties 'drawn', 'problems'`.

The host now shares its created-document answer between agent New and routed New. For Diagrams it asks the new window's documented set-text action to read back drawing status for its existing text, preserving the existing output envelope and recovery flush check. Other applications retain their list-row answer.

The second defect concerned language selection. In this package, Diagrams `openPath` reads the resource text and then calls `openInTab(text, resourceIdentifier, null, view)`. `makeTab` calls `kindOf` on the identifier and text. The display name from the read response does not control that initial kind. Dinah returned identifiers such as `Untitled.canvas (not saved)` and `Untitled.mmd (not saved)`, which hide the extension. Empty Canvas and invalid Mermaid text therefore became D2. Naming the draft alone did not fix this.

New resource assignments now put the unsaved label and collision number before the extension. Both New paths give Diagrams a name for the requested kind, defaulting to D2 when omitted. Two new canvases return identifiers `Untitled (not saved).canvas` and `Untitled (not saved) (2).canvas`, both with kind canvas. The invalid Mermaid request returns kind mermaid, drawn false and a problem stating that no diagram type was detected for `not valid Mermaid`.

Existing legacy identities are retained so recovery snapshots still resolve them. This change fixes new assignments; it does not migrate old diagrams that were already assigned a legacy identity or repair their previous kind selection.

## Survey and verification

Applet discovery was performed for Diagrams, Docs and Slides. Live invocation was not attempted because New opens a visible document window and has no hidden-target option. Lead confirmed the headless substitution. The fixture runs the actual bundled applets and Dinah page routing with an in-memory draft bridge at an intercepted HTTPS test origin. It invokes no native UI, OS input or real filesystem document writes. No scratch documents were created outside the fixture. The browser is explicitly headless in the shared launcher.

The fixture collected 19 responses across these 14 host-handled action references. All 19 candidate outputs pass both the packaged browser schema validator and Dinah's Go validator.

| Application | Host-handled actions checked | Schema result |
| --- | --- | --- |
| Diagrams | new, open-file, open, save with path, close | All match after correction. New also covers repeated Canvas names, invalid Mermaid, explicit/default D2 and documentWindows.open. |
| Docs | new, new-template, open-file, open, save with path, close | All match. new-template's schema requires only an object. |
| Slides | present, save with path, close | All match. present is the handled document-opening action; this fixture does not start a native show. |

Slides declares no new, new-template or open-file action. Its open action is a live action forwarded to the package rather than one answered by these host interception branches. Other forwarded actions and export formats were outside the narrowed survey. A matching schema does not certify every action's document semantics, save failure behavior or asynchronous close completion.

The baseline host files were byte-identical to the base revision's Git blobs before and after the run: content-host `60e014847cdb373b78dbe350772baa0904e8d7d5` and content-windows `764fed8c5ed16700befee09f399c3779927b013a`. The baseline run failed the two Canvas kind checks, invalid Mermaid kind check and routed New schema check. Its Go receipt check also failed on the missing drawing fields. All unfavorable evidence remains retained.

The final focused browser run exited 1 in 8.697 seconds. All schema, requested-kind and legacy-identity assertions passed, but its page-error assertion failed on two uncaught language-service cancellations. This is not a clean browser pass. The desktop Go batch exited zero: `TestContentHostActionOutputs`, `TestTheContentApplicationsActionsAreOfferedWithDinahsCapabilities` and `TestAnActionThatChangesADocumentIsAbsentWhereAgentsPropose` passed (package time 0.267 seconds). The same filtered command included internal/applet, which reported no tests to run; the desktop receipt test calls its actual Validate function. ESLint reported no errors in the two production JS files; the e2e file is excluded by the repository's lint configuration. `git diff --check` passed.

Early fixture runs used the shared helper's HTTP test origin. Slides could not initialize there because `crypto.randomUUID` was unavailable. Using a locally intercepted HTTPS origin corrected the fixture without changing the package. The failed HTTP receipts and console diagnosis remain available.

## Remaining package failure

The secure baseline and candidate runs both emit two unhandled promise rejections around the Diagrams close portion of the survey:

```json
{"code":-32800,"message":"Language session ended."}
```

The source of this exact packet is packaged `content/md-editor/language-service/bridge.js`, `endRequests`: it rejects pending initialize requests when a language session ends. Diagrams calls the language client's dispose method during teardown. The observed cancellation escapes as an unhandled rejection. The package needs to consume expected initialization cancellation on disposal, including derived initialization promises, while preserving failures for live sessions. An upstream regression should close a Canvas window while language initialization is pending. This review identifies the packet's origin and observed failure; it does not claim to have isolated every promise chain responsible. Packaged files were not edited and the headless test still fails on these errors.

## Evidence and reproduction

Evidence is in the following temporary folders, each containing outputs.json. The final folder also contains console.json and rejections.json.

- `%TEMP%/dinah-content-output-baseline`
- `%TEMP%/dinah-content-output-candidate`
- `%TEMP%/dinah-content-output-diagnostic`
- `%TEMP%/dinah-content-output-console`
- `%TEMP%/dinah-content-output-secure`
- `%TEMP%/dinah-content-output-final`

Run the focused browser suite through scripts/with-test-lock.ps1 with Node and `cmd/dinah-desktop/frontend/e2e/content-action-outputs.js unused <output-folder>`. The e2e dependencies must be available; this work used NODE_PATH pointing to the main checkout's existing e2e/node_modules. The optional DINAH_CONTENT_HOST_ROOT selects an alternate directory containing the two host source files for baseline comparison.

Set DINAH_CONTENT_OUTPUTS to the resulting outputs.json, then run `go test ./cmd/dinah-desktop -run '^TestContentHostActionOutputs$' -count=1` through the same FIFO wrapper. With no receipt specified the Go test skips explicitly. The browser suite independently validates outputs and fails on schema mismatches and page errors.

No full browser suite, live applet action, installation, merge or push was performed. Native operation, actual disk persistence and recovery across a real product restart were not verified.
