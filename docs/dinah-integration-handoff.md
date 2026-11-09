# Dinah integration handoff

This handoff is current as of November 9, 2026. The Dinah integration agent and Dinah Team Lead should use it to adopt the portable Editor, Docs and Diagrams applets into Dinah Desktop. Slides ships in the same dependency set and remains a compatibility consumer; adding its production UI is optional unless Dinah needs it.

The editing extraction is implemented, merged and installed in yavc.app. An isolated build of Dinah's actual Windows application has passed native acceptance. Integration with Dinah's production shell, existing recovery owner and document workflows remains to be implemented and tested. The proof has not replaced the installed Dinah executable or migrated personal documents.

This document supersedes the version inventories and readiness statements in the older [editing integration brief](dinah-editing-integration-brief.md). The [application design](content-applications-design.md) and [portability design](applet-portability-design.md) retain the architectural decisions. Earlier contracts sometimes describe their original experimental stage; use their operation/version definitions together with the current [workspace contract](content-workspace-contract.md), rather than treating historical progress notes as the current backlog.

## Release to adopt

The receiving agent should begin with one pinned release set and preserve its complete manifest. A source branch name or an individual applet version does not identify that set.

| Item | Verified identity |
| --- | --- |
| Producer repository | `https://github.com/paulmooreparks/yavc.app` |
| Release | `v0.2.49` |
| Release commit | `1f6bbf52c66bcfa60d6ebf84e5788397340d7623` |
| Feature implementation commit | `d10b054c27a9e67859afa08ee46679f8a57894bd` |
| Merged changes | [PR #59](https://github.com/paulmooreparks/yavc.app/pull/59), [PR #60](https://github.com/paulmooreparks/yavc.app/pull/60) |
| Portable manifest digest | `739173773e8fc5ff1437ade501e05e88bf03e023b97f9d51dd9f4f0dd7b9684a` |
| Manifest file count | 483 |
| Served content assets | 274 |
| Applet versions | Editor 46.13.0, Docs 1.13.0, Diagrams 0.31.0, Slides 0.41.0 |
| PUDL | 0.105.0, bundled verified distribution |
| Published site image | `ghcr.io/paulmooreparks/yavc.app:v0.2.49` |
| Published image index digest | `sha256:79bd6444f04dbab199ade933171ff2013d06142455ac53fa0e6e33d786dad735` |
| Image workflow | [Successful publication](https://github.com/paulmooreparks/yavc.app/actions/runs/42570408569) |

The site image identifies the release and its reintegration. Dinah should consume the portable package, not run the yavc.app server or import its guest filesystem. The manifest digest identifies the ordered asset inventory; the manifest records each file's length and SHA-256. Verify both the aggregate digest and every file before mounting it.

## What the applets provide

Editor and Docs are separate PUDL applets backed by the same maintained workspace implementation. Their identity must remain stable when the user changes views. Editor defaults to source editing and provides read-only Markdown preview. Docs opens Markdown for rich editing, defaults ordinary `.md` to Visual and `.docs.md` to Document, and retains backing-source access. Code such as Go or C++ belongs in Editor. Office import creates a native copy and reports losses; it never changes Save into an Office-container writer.

Docs now includes physical page layout, paragraph/list/table/container pagination, continued page notes, styles, typography, headers/footers, explicit review suggestions and comments, and Word/PDF export within documented format limits. Review is persisted annotation editing, not automatic tracking of ordinary keystrokes. Editor includes source history with mixed line endings, negotiated encoding-aware saves and bundled language assistance, including CSS and HTML.

Diagrams provides D2, Mermaid and JSON Canvas. Canvas has visual editing, multiple selection, connectors, automatic anchors, source linking and an Inspector. D2 has Outline/navigation and copy conversion to/from Canvas with loss review. Mermaid remains source editing with rendered preview. Embedded and saved-reference diagrams work in shared Markdown consumers. DOCX/PDF diagram export remains unsupported and is refused explicitly.

The native Obsidian probe also verified bidirectional Canvas saves. Automatic anchoring now uses additive metadata alongside standard endpoint sides. Preserve nested unknown JSON fields and the [Canvas extension contract](canvas-extension.md); do not normalize this metadata away in a host adapter.

## Boundaries the production adapter must retain

Dinah owns resource identity, workspace access, permissions, native drafts, conditional persistence, attachment resolution, recovery, shell windows and publication. The applets own editing transactions, source/rich synchronization, rendering and their command implementations. Adapt Dinah's existing services instead of introducing another filesystem, draft database or publication authority.

| Need | Production integration requirement | Contract or reference |
| --- | --- | --- |
| Existing-resource access | Map opaque host IDs to authorized resources. Read returns a baseline; save compares that baseline atomically at the native authority. | [Resources](content-resource-contract.md) |
| Encodings | Advertise version 1 only for the UTF-8 contract. Implement version 2 before promising UTF-16 or encoding/signature preservation through native copies. | [Encoded resources](encoded-resources-contract.md) |
| New and copied resources | Implement optional create-only destinations and host-owned Save As through Dinah's existing creator/picker. Preserve atomic absence or destination-revision checks. | [Creation and Save As](content-creation-contract.md) |
| Recovery | Supply snapshot version 2. Retain original baselines, unbound drafts and conflicts. Acknowledgement must mean durable storage has completed. | [Recovery](content-recovery-contract.md) |
| Document windows | Supply `documentWindows` version 1 for one document per host window. Omitting it intentionally retains tabs. Wait for readiness and recovery acknowledgement before releasing transferred source. | [Document windows](document-window-contract.md) |
| Workspace services | Supply dependency bases and optional shortcode catalogs through the workspace service. Keep preferences and resource storage separate. | [Workspace](content-workspace-contract.md) |
| Selection and attachments | Route file selection, computer upload/import and document-relative dependencies through authorized native services. Persist an imported attachment before accepting its path. | [File fields](file-fields-contract.md) |
| Action capabilities | Register the project-defined `files.read` and `files.write` capabilities before package installation. Expose bounded services; execute grants still apply. | [Workspace action registration](content-workspace-contract.md) |
| Chrome and lifecycle | Map command/menu/toolbar contributions, title/dirty state, notices, readiness, navigation and disposal into the owning Dinah document window. | [Portability](applet-portability-design.md) |

The application contexts may expose conditional writes. A read-action capability context must not expose the resource writer. The proof registers both known capability names with bounded read services and leaves writes behind the conditional resource boundary. Registration does not grant invocation rights; inspect `pudlActions.problems()` and exercise public actions during acceptance.

The production adapter must synchronize a pending rich edit before Save, Open, New, view ownership changes or closure. A save acknowledges only its captured resource instance and submitted edit revision. Later typing remains dirty. A stale completion cannot update another window or a reopened resource. A conflicting or failed write retains both the local draft and the host's competing draft.

The native proof serializes one complete UI-state owner. Dinah's real shell already has an owner, so copying the proof's whole-store `SaveUIState` loop would risk overwriting unrelated state. Integrate applet recovery and preferences with that existing owner. The snapshot contract has a synchronous `flush()` result; an asynchronous native write cannot report success until its acknowledgement has arrived. Resolve this lifecycle explicitly with the Team Lead before allowing a production close or migration to release a draft.

## Existing native proof

The proof uses a clean detached Studio checkout at `ba2b8b72008e53fc1b8601f5d903ce95fa53558d` and a clean extension checkout at `444129580fb6f8f8d3ce3248a09f8df97c6699ac`. Its adapter calls Dinah's `SetWorkspace`, `ReadWorkspaceResource`, `OpenDraft`, `GetDraft`, `UpdateDraft` and `SaveDraft` methods against synthetic files. These names identify the tested baseline, not a promise that newer Dinah revisions have unchanged signatures.

Read these sources before implementing the production adapter:

- `scripts/fixtures/dinah-content/native-host.js` supplies the bounded proof adapter.
- `scripts/fixtures/dinah-content/native-checks.js` exercises the native resource and applet behavior.
- `scripts/prepare-dinah-content-proof.cjs` validates the artifact and prepares an isolated Go overlay.
- `scripts/build-dinah-content-proof.ps1` pins receiver and extension revisions and builds the proof executable.
- `scripts/test-dinah-content-native.cjs` runs the actual application with synthetic workspace/profile state and records acceptance.
- [Native acceptance record](dinah-desktop-acceptance.md) explains the method, original failures and security boundary.

The latest run used actual Edge WebView2 157 and verified all 483 artifact files. It passed native source and rich saves, review resolution and Undo, Canvas extension retention and save, D2 worker/WASM, CSS/HTML language workers, page-note measurement, competing-draft refusal and disposal/recovery persistence. The window remained hidden. Shared-desktop focus, operating-system dialogs, Linux/macOS webviews and the production shell were not exercised. The native adapter advertises resource version 1 only.

The page keeps Dinah's authenticated native bridge and restrictive CSP. D2 needs a narrowly scoped worker response policy allowing its bundled layout evaluation. The proof changes only that exact worker route; it does not enable page-wide eval or an unrestricted native bridge. Package assets and editable document dependencies require separate resolvers. Do not make CDN requests or network fallback part of offline startup.

## Evidence on Paul's current machine

These paths are evidence locations, not durable distribution URLs. Copy the manifest, complete artifact and receipts into an integration-owned location before a temporary directory is removed.

| Evidence | Location |
| --- | --- |
| Complete portable artifact | `%TEMP%/yavc-canvas-interop-artifact` |
| Prepared native proof | `%TEMP%/yavc-native-canvas-interop` |
| Successful committed-source native run | `%TEMP%/yavc-native-canvas-interop/run-d832d0a9-760a-4dcf-a1b7-4ef5da23177a/acceptance.json` |
| Native build identity | `%TEMP%/yavc-native-canvas-interop/build.json` |
| Clean receiver proof checkout | `%TEMP%/yavc-dinah-native-proof` |
| Clean extension proof checkout | `%TEMP%/yavc-dinah-extension-proof` |
| Native Obsidian receipt | `%TEMP%/yavc-obsidian-committed-acceptance/receipt.json` |
| Installed candidate receipt pointer | `%TEMP%/yavc-canvas-interop-backup-path.txt` |

The last pointer identifies the backup directory containing `verification.json`, `installation.json`, `release.json`, copied native/Obsidian receipts and installed-browser logs. Earlier failed probes remain separate from passing runs. The maintained Studio and yavc.app checkouts contain unrelated edits and must not be reset or cleaned to reproduce this proof.

## Reproduce before integrating

Use an isolated producer checkout at the release commit and fresh output paths. The complete release builder includes licenses, support modules, PUDL, workers, fonts, WASM and both unrelated-host fixtures. Do not cherry-pick applet scripts or mix shared engines from another release.

```powershell
$artifact = Join-Path $env:TEMP 'dinah-integration-content-release'
go run ./cmd/yavc-content -out $artifact
node scripts/test-portable-workspace.cjs $artifact chromium firefox webkit
node scripts/test-portable-diagrams.cjs $artifact chromium firefox webkit
```

The builder refuses existing output and output inside the checkout. The browser tests require Playwright and its installed engines. Compare the resulting manifest with the pinned digest above. If it differs, investigate source/toolchain inputs and record a new reviewed candidate; do not relabel differing bytes as this verified artifact.

The native proof builder intentionally refuses a dirty receiver or a different receiver/extension revision. Create clean isolated checkouts at the recorded commits for baseline reproduction. A port to current Dinah requires a reviewed update to those pins and overlay mappings.

```powershell
pwsh -NoProfile -File scripts/build-dinah-content-proof.ps1 `
  -Receiver $receiver -Artifact $artifact -Proof $proof -Extension $extension
pwsh -NoProfile -File (Join-Path $receiver 'scripts/with-test-lock.ps1') `
  node scripts/test-dinah-content-native.cjs $proof
```

Set `$receiver` and `$extension` to those clean checkout roots, and `$proof` to a new isolated output directory. The runner creates new synthetic workspace/profile directories for each execution. `-Prepared` is available only for verified reuse of an existing prepared proof. Retain `preparation.json`, `build.json`, fixture hashes, executable hash, logs and each `acceptance.json`.

## Production integration sequence

1. The integration agent and Team Lead compare current Dinah with the tested baseline. Agree on the first production surface, native ownership, dependency version and existing behaviors that must remain available. Start with Diagrams if the document-editor replacement would otherwise block a bounded integration.
2. Package the complete release as a maintained upstream dependency with a recorded manifest and rollback version. Serve its immutable assets through Dinah's approved asset route and map CSP/content types without broadening unrelated routes.
3. Implement the production host adapter against Dinah's resource/draft authority. Add native creation, Save As, attachments and recovery ownership where needed. Features whose host capabilities are absent must be disabled with a reason or made read-only.
4. Integrate single-document window allocation, title/dirty state, focus, command routing and unsaved-close behavior. Keep Docs, Editor and Diagrams independently identified and scoped. Opening an existing destination raises it; successful Open must remain foreground after picker dismissal, while Cancel retains the opener.
5. Run receiving-host acceptance against copied documents in an isolated profile, then compare the result with Dinah's existing editor workflows. Record any unsupported behavior and obtain the product decision before removing a useful existing path.
6. Stage an installable candidate with its previous executable, package, adapter and recovery data available for rollback. Installing or replacing the person's running Dinah requires its own authorization. This handoff does not authorize that replacement, production deployment or migration of real data.

## Production acceptance checklist

The integration is ready only when the actual receiving shell passes these checks with recorded source revisions, runtime version and artifact hashes:

- Editor, Docs and Diagrams mount together and open multiple independent documents without sharing source, history, selections, diagnostics or mutable state.
- Source and rich edits save through native conditional writes; newer edits remain dirty after delayed acknowledgements, and conflicting native drafts remain recoverable.
- New, Save As, import and attachment selection honor native destination permissions, cancellation, display-name validation and competing destination changes.
- Recovery survives app restart with the original baseline, unbound drafts, exact supported source format and each document's selected view. Failed durable recovery prevents closure or transfer from claiming success.
- Document Open raises new and existing destinations after modal cleanup. Cancel retains the opener, and late callbacks do not steal focus after the user changes windows.
- Native window titles show the active document and unsaved state even without tabs. Commands, menus, shortcuts and dialogs resolve the owning document instance.
- Untouched source and unknown supported metadata survive opening, visual editing, saving and recovery. Unsupported rich constructs remain safely source-editable.
- Page layout, editable notes and review resolution work in Docs, while Editor's preview remains read-only. Import and export preserve originals and expose their declared losses.
- All assets load offline under the receiving webview policy. D2/WASM and JSON/Canvas/CSS/HTML workers work without page-wide eval, external network fallback or file-authority bypasses.
- Disposal releases owned dialogs, subscriptions, workers and temporary resources. Delayed reads, saves and analysis cannot mutate a closed or different document.
- The prior package and adapter can reopen retained native resources/recovery without data loss. Any state-schema migration has an explicit compatibility or rollback plan.

Windows proof success does not establish Linux or macOS native acceptance. Run the same behavior checks in each platform Dinah intends to ship. The separate PUDL numeric increment request remains upstream work and does not block adopting the verified editing package.
