# Dinah title launcher

Status: implemented and consumer checks passed in the integration worktree. The installed desktop remains unchanged.

Paul requested adoption of PUDL 0.102.0, conversion of the Dinah site-menu title into the launcher, and relocation of Settings and Quit Dinah to the top of the task bar. Existing title-menu commands must remain available elsewhere. This work preserves the installed window-stack and live-work changes.

## Command placement

The proposed Workspace menu contains New Chat, New Workspace, New Empty Workspace, Rename Workspace, Export Workspace, Import Workspace and Recent Workspaces. Install Packages moves to Tools because it installs application capabilities rather than operating on a workspace. Existing enabled-state checks, command identifiers and handlers remain authoritative. The launcher lists available destinations through the existing workbench catalog and uses documented PUDL activation contracts.

The current taskbar is horizontal at the bottom of the desktop. The initial interpretation of top is its leading command area, before window buttons. The placement must remain usable at narrow widths and must not collide with the status tray. Settings and Quit keep their existing handlers, including persistence flushing before Quit. A destructive Quit action must never be invoked simply by opening the launcher or navigating its list.

## Integration dependencies

On 6 November, GitHub release lookup for v0.102.0 returned release not found. No candidate assets have been substituted and neither vendor folder has changed for this request. Parkscomputing Development has been asked for its implementation pattern, documented APIs and known pitfalls. PUDL Team Lead has been asked for release provenance, title-launcher and taskbar placement contracts, and integration concerns. Their replies arrive asynchronously.

The implementation must retain readable and minified distribution parity in both vendor folders. Verification covers title activation by mouse and keyboard, folded-menu access, launcher search and destination activation, focus and dismissal, saved presentation state, relocated command availability, taskbar overflow and existing window-stack persistence. Tests must exercise real PUDL controls. A new desktop installation is a separate step from this implementation request.

## Parks Computing integration review

Parkscomputing Development supplied its host integration on 6 November. Its beta uses candidate 8ed0119 based on 0.101.0; this is reference material, not release provenance for 0.102.0. The primary reference files are Desktop.cshtml, _Layout.cshtml, _MenuBar.cshtml, wwwroot/js/launcher.js and taskbar-pins.js under Application/parkscomputing-engine in the parkscomputing repository. Tools/test-launcher.js and test-taskbar-pins.js provide consumer test examples.

The reported public integration gives the real menubar data-menubar-launcher pointing at the catalog ID and retains data-menubar-id="site" on the identity entry. The folded entry uses Open launcher. The catalog is a named .start-menu[data-start] with search, category and item controls. PUDL Start is enhanced before Launcher. The launcher obtains its invoker through pudlMenubar.launcherButton() and its visible anchor through launcherButton({ visible: true }); retained references to generated menu buttons are unsuitable when the menu folds or rebuilds. These contracts must be checked against the released documentation before implementation.

The shared catalog can move between popup and pinned pane. Its workspace split excludes the taskbar. Dinah will keep presentation and size preferences in host UI state; any shareable launcher state must integrate with its saved URL without replacing window state. First-visit pinned presentation is a Parks Computing policy and is not yet adopted as Dinah's default.

Managed destination actions call pudlLauncher.activated synchronously when accepted, then run through their normal handler. Dinah must not delay or replay clicks. Modified navigation remains native. Keyboard tests cover named search and categories, result navigation, focus return and pin state. Escape must respect handled events and nested popovers before closing a pinned catalog. Narrow layouts must leave the activated window reachable.

Parks Computing does not establish the requested taskbar command placement: its Settings command is in the launcher header and the website has no Quit equivalent. Dinah's existing valid region navigation must be retained; Parks Computing's removal of a legacy region handler does not justify removing Dinah's handler.

The release lookup still reports v0.102.0 unavailable. Vendor adoption and code dependent on that release remain pending the upstream response. No desktop build or installation occurred for this launcher change.

## Preparation and pinning dependency, 7 November

PUDL Team Lead confirmed that 0.102.0 is not published and that d88c71e is mutable release preparation, not adoption provenance. The documented fixed-command placement is ordinary glyph-bearing buttons directly in .win-bar before nav.win-dock, outside destination registration. Standard trailing window controls remain intact. Dinah has prepared these buttons and the Workspace menu in source using existing command handlers. Settings and Quit remain temporarily available through the old identity menu until the launcher replaces it. Install Packages has moved to Tools. These preparatory edits are not installed.

Paul subsequently reported a pinning problem found by Parkscomputing Development. A request for reproduction, affected revision, ownership and regression coverage is pending. Pinning integration remains held until the finding is understood. The independent menu/taskbar command test is queued behind upstream's shared browser test lease.

Parkscomputing Development clarified that the finding concerns taskbar destination pinning, not launcher-pane pinning. Its unpublished 8ed01196076115b0299bcfc6c6bbf35033fb7be8 adapter only registered selected destinations, causing other articles to lack Pin; picture replacement also involves its adapter and a reported descriptor contract gap for dormant picture pins. Those findings do not establish a defect in a published 0.102.0. The title launcher and its pane can proceed independently when the release is ready. Taskbar destination pinning remains outside this implementation.

If Dinah later adopts destination pinning, its review must cover category-based eligibility with explicit opt-out, permissions, two distinct destinations in one category, and picture preservation while running, dormant and restored, including failed-image fallback. These are separate from launcher-pane presentation checks. The upstream/consumer issue record is Architecture/taskbar-pins-beta-integration.md in the parkscomputing repository.

The preparatory launcher-commands browser suite passed against bundled PUDL 0.101.0. It verifies command relocation, Settings activation, native Quit dispatch, and fixed command visibility and non-overlap at 1280 and 640 pixels. Evidence is build/launcher-commands-final. The earlier test incorrectly waited for the hidden Settings heading; the corrected assertion checks the visible Settings window. The launcher runtime and 0.102.0 adoption are still pending, and no build was installed.

## Adopted behavior, 7 November

Paul explicitly chose a closed, unpinned launcher for first use. The title opens a popup. A pinned pane is restored only after the person explicitly pins it. This differs from the Parks Computing first-visit policy. The browser regression checks the initial closed presentation.

Both vendor folders now contain all 193 distribution files from released PUDL 0.102.0, commit f8a1f70e6ccee92cc6245cf3e685499326c45ca3. The archive SHA-256 is 1b7db18e6bd330b3eac4461cf684ef74064d1cff09f604d1b3234471bae715f3. Each file was compared with the immutable tag. The earlier release-unavailable entries above describe preparation before publication.

The canonical menu is named Workspaces to distinguish it from the active Workspace applet menu. Settings and Quit use their existing command handlers from fixed taskbar buttons. The title and folded menu share the PUDL launcher toggle. Presentation, dimensions and splitter widths use host UI state; search, category and presentation also participate in the saved URL.

Launcher, relocated-command, UI-state and window-stack browser checks passed. The window-stack checks use fresh browser contexts and delayed applet loading; they do not establish a native desktop restart result. A production desktop build succeeded, but it has not been installed. Narrow taskbar verification remains in progress.

## Verification update

The first-visit closed/unpinned assertion, launcher interaction suite and relocated-command suite passed in build/title-launcher-complete. Narrow taskbar checks now pass at 390 pixels after reducing host command widths and gaps; every fixed command and the active window remain visible. The broader workbench suite still fails waiting for the Ribbon tabs chooser, both at its original 560-pixel fixture and a 390-pixel probe. Its original expectation is retained pending investigation. This is an open integration gate.

The final source builds successfully as build/title-launcher-complete/dinah-desktop.exe, version 0.0.0-title-launcher-20261107. No installation or native restart was performed.

## Ribbon investigation resolved, 7 November

The missing Ribbon tabs chooser was an outdated test expectation. Live browser inspection showed the ribbon enabled and its chooser visible from 600 through 1000 pixels in the initial fixture. At 560 pixels the complete menubar had collapsed, which intentionally disables the ribbon until more room is available. The additional Workspaces menu changed the width at which this transition occurs. PUDL computes the chooser only while the ribbon is active and menu titles have folded to glyphs. No product code change was required.

The workbench regression now exercises the intermediate stage at 700 pixels, opens the chooser by keyboard, verifies Escape restores focus, checks full collapse at 390 pixels, and verifies widening restores the ribbon and chooser. The complete workbench suite passes. Evidence is build/ribbon-investigation/state.json and build/ribbon-investigation-verified. This resolves the open integration gate described above. The desktop build remains uninstalled.

## Installation, 7 November

Paul authorized installation for testing. Installed build 0.0.0-title-launcher-20261107 includes PUDL 0.102.0. The installed executable matches the tested build SHA-256 9616C50A70CD71C652FA9EA302F9929B5973700EC50127B7CE619D8DC8608079. The previous executable is retained at C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-title-launcher-20261107-042004. The installation receipt is build/title-launcher-complete/install-receipt.json. Running desktop process IDs were unchanged; no restart was performed. Paul will restart and test the launcher.

## Launcher command expansion, 7 November

Paul requested a Terminals category containing every former Tools > New Terminal command, removal of Tools, and Settings and Quit in the launcher header instead of the redundant Dinah heading. The source implements these changes. Existing taskbar shortcuts remain available because their removal was not requested. The launcher keeps its accessible name through aria-label.

Terminals contains Default Shell, each detected shell and Refresh Shells. Explicit choices retain the current workspace path and shell identity. Refresh rebuilds the catalog from the shell provider event. Other former Tools destinations remain in the catalog; Install Packages and Find a Tool have been added so no Tools command is lost. Settings and Quit use the existing command registry and shutdown guards. These follow-up changes are not installed.

### Further command placement recommendations

The following changes are recommendations for review and have not been implemented.

| Commands | Recommendation | Reason |
| --- | --- | --- |
| Skills, In Progress, Notifications | Copy into launcher categories. | These open useful destinations that should be searchable without knowing a menu. |
| New Workspace, New Empty Workspace, Import Workspace | Copy into Workspace. | These begin work and belong alongside destinations. Keep the Workspaces menu for now. |
| Recent Workspaces and Recent Documents | Add dedicated recent groups with real navigation targets and workspace labels. | These let the launcher resume work while preserving identity and navigation behavior. |
| New Document and Open Document | Copy into a Documents category. | These begin document work; the generic Editor entry can be renamed to clarify whether it creates a blank document. |
| Keyboard Shortcuts and About | Copy into a Help category. | Searchable help is useful; the small Help menu can remain for familiarity. |
| Theme Editor, color mode and themes | Keep Theme Editor searchable; consider an Appearance category only if direct theme selection is wanted. | Listing every theme in All would add clutter. Settings already offers configuration. |
| Rename or Export Workspace, Save, Save As, Close, Discard | Keep their canonical context menus. | Their targets and availability depend on the active workspace or document. A later command-search mode would need to state the target explicitly. |
| Window arrangement, Stop, Restart Agent and model selection | Keep in Window and active-app menus. | These operate on current work and need visible context, state and safeguards. |

I recommend the destination and creation copies first. Removing more canonical menus should follow user testing of those entries, rather than leaving contextual operations hidden among launcher destinations.

### Approved destination and creation additions

Paul said to proceed. The launcher now includes Skills, In Progress, Notifications, New Workspace, New Empty Workspace, Import Workspace, New Document, Open Document, Keyboard Shortcuts and About. Documents and Help are new categories. New Document and Open Document replace the ambiguous generic Editor entry. Existing canonical menus remain, except for the explicitly removed Tools menu. Command availability follows registry updates and is checked again when invoked.

Recent-work groups remain a follow-up recommendation pending their navigation and workspace-label design. Appearance and contextual commands remain in their existing menus. The focused shell/category/header test, title-launcher test, command-placement test and complete workbench suite pass. The tests use the isolated desktop bridge fixture; native shell processes and actual application shutdown are not invoked. Evidence is build/launcher-expanded and build/launcher-expanded-final. These changes have not been installed.

## Expanded launcher installation

Paul authorized installation. Build 0.0.0-launcher-terminals-20261107 was installed with PUDL 0.102.0. The installed SHA-256 is BC2CF85D352113B4E8AF0B07FA16B0706C4DEB9A453A15405FF0A9C8BCE07008. The prior executable is retained at C:\Users\paul\.dinah\bin\dinah-desktop.exe.before-launcher-terminals-20261107-044729. The receipt is build/launcher-terminals-verified/install-receipt.json. No restart was performed.

## Taskbar command duplication removed

Settings and Quit belong in the launcher header. The duplicate fixed taskbar buttons have been removed at Paul's request. Both header commands retain their existing handlers.


The taskbar cleanup was built and installed on 2026-11-07T12:23:07.5667359+08:00. The installed executable matches SHA256 0F15215AFED93A7A4B164B9F17D9936AE37B50A191A11109D91D491A69605190. Rollback executable: C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-taskbar-cleanup-20261107-122307. The focused launcher command check passed. Dinah was left running; restart remains with the person.

