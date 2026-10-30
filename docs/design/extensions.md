# Extension implementation contract

Paul authorized implementation of `docs/plans/extensions.md` on 25 October 2026. This document records implementation decisions and follows the canonical [Architectural Principles](C:/Users/paul/OneDrive/Documents/Architectural%20Principles.md). The extension SDK and Git applet live in separate repositories and compile without Studio source.

## Trust and transport

Installing a signed native extension trusts its publisher to execute native code under the user's account. The process job controls lifetime; it is not an operating-system filesystem or network sandbox. Permissions constrain host API calls, and sandboxed frames isolate frontend code. The installation screen must state this boundary. Signature validation authenticates package contents and the pinned publisher key, without certifying publisher identity or code safety.

The backend uses standard MCP over stdio. Host callbacks use a documented authenticated HTTP JSON interface because MCP server tools do not imply reverse client tools. Each tool invocation receives an unpredictable, expiring host-call scope through namespaced MCP `_meta`. Its workspace, extension identity, permissions and effect classification are established by Dinah, never by frontend arguments. A scope expires when the invocation returns or is cancelled. Callback URLs accept only loopback traffic, bearer authentication and bounded bodies; the frontend never receives a backend scope.

The signed manifest contains actions and their schemas, capabilities and replay policy. MCP tools must match this declaration before use. Native-process actions are conservatively effectful for agent approval because a general process permission cannot prove a command is read-only. User-triggered frontend actions use the same action implementation, with explicit confirmations for destructive Git operations. Process execution uses executable and argument arrays without a shell, bounded output, deadlines and workspace/source-root confinement. The Git extension records every command and supports preview before execution.

## Packages and lifecycle

Packages contain a strict versioned manifest and a canonical SHA-256 file inventory signed with Ed25519. The inventory covers every regular payload file; symlinks, path traversal, duplicate paths, unsupported versions and oversized packages are rejected. Installing copies a validated snapshot without executing it. The active installation and pinned key are committed atomically under a shared file lock. An existing ID cannot silently change publisher keys. Installed files are revalidated before backend launch and frontend serving. Updates stage a new signed version before activation; running instances are stopped and not retried across a possibly effectful tool call.

The first failed connection can be restarted for a later request, once per host lifetime. An action interrupted by a crash fails and is never transparently replayed. Disabled or removed extensions cease contributing actions and windows. Installation and permission changes take effect on a fresh registry without replacing an invocation's running registry.

## Frame contract

Frontends use sandboxed frames with scripts permitted and same-origin privileges omitted. Neither cookies nor the native bridge are exposed. Host asset routes serve only verified extension files and approved PUDL assets, with a restrictive CSP and font CORS suitable for opaque origins. The host binds a fresh MessageChannel to the expected frame on each load. Navigation or disposal invalidates pending calls and the channel. Serialized origin `null` alone is never an identity.

The host sends a versioned appearance snapshot before revealing a frame, with resolved theme, allowlisted token values, fonts and direction. Frames do not load independent theme-preference code. Commands cross the channel as bounded data descriptors rather than functions. Navigation state belongs to the host workbench URL. Local shortcuts stay in the frame; only declared host navigation commands cross the boundary. Focus activates the owning window.

## Git proof

Git uses documented command output, argument arrays and pathspec separators. Mutations are serialized per repository. Staging a hunk validates the displayed diff revision before applying the original complete hunk, and never constructs a patch from stale line numbers. Status paths use NUL delimiters so spaces, tabs, newlines, renames and leading option characters remain distinguishable. The command log records arguments, explanation, output and exit status. Terminal handoff opens a prepared command without executing it.

PUDL Development recommended master-detail, trees, grids and split panes as existing controls. Hunk checkboxes remain separate from native code-text selection. Git topology and operations stay in the extension. Potential reusable diff and externally managed appearance controls will first be implemented and tested in Dinah before any PUDL release.

## References

The [MCP tools specification](https://modelcontextprotocol.io/specification/2025-11-25/server/tools), [iframe sandbox contract](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/iframe), and [Git porcelain status format](https://git-scm.com/docs/git-status) define the external behavior used here. Implementation and verification status will be recorded as each part lands; this design document does not certify unimplemented capabilities.

## Form interaction

Extension frames allow scripts and forms while retaining an opaque origin. The HTML response always sets `form-action 'none'`, so an extension can use normal submit events and input validation without sending a browser form request. Omitting `allow-forms` prevents those events and broke the Git commit form in the native integration test. This uses the documented [iframe sandbox form permission](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/iframe) and [CSP form-action directive](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/form-action).

## Published proof and verification

The SDK is published as `github.com/paulmooreparks/dinah-extension-go` v0.1.2, with source revision `4cded56`. The Git extension is published as `github.com/paulmooreparks/dinah-git` v0.1.0, with source revision `0ff5b10`. Its signed amd64 package contains Windows, Linux and macOS backends. Both repositories passed Go tests and vet in GitHub CI on all three platforms. The SDK also tests stale focus tokens, failed menu entry and unusable return controls. No Studio source replacement remains in either consumer's Go module.

The authenticated Windows host smoke test uses a hidden isolated Dinah instance, a disposable state directory and disposable Git repositories. It exercised repository discovery through the actual MCP backend, original diff rendering, a workspace-pinned diff window, complete-hunk staging, a reviewed-index commit, history, actual command logs, prepared Terminal handoff, Show Me First cancellation and a 390px viewport. It passed without page errors. This verifies the native host and browser renderer path; it does not establish native macOS/Linux desktop integration or native WebView2 composition behavior.

All 72 Dinah browser suites passed together. Extension-specific Go tests cover signed backend startup, expiring callback authority, schema precision, old-discovery rejection after updates, damaged-package isolation, workspace file boundaries, stale file revisions, asset integrity/CSP, and bounded expiring prepared-command tickets. Vet passed. An early full Go run passed; later full runs, including a serial run, hit the existing `TestClosingATerminalEndsItsShell` timeout. That test passed alone. No terminal-lifetime improvement is claimed by this extension release.

The reusable PUDL addition is `pudlMenubar.focus({returnTo})`, plus the `pudl:menubar-return-focus` event. Both Dinah vendor copies contain the same candidate. Return targets are revalidated for connection, visibility, disabled state and inert ancestors; an event is sent only after focus reaches the target. Bar titles and active panels share one excursion. PUDL Development reviewed the initial candidate, and its reported lifecycle findings were corrected. Upstream adoption is separate from this Dinah change.

A frame may invoke `dinah.hostCommand('open-view/<view-id>')` to open one of its own declared views with its current addressable state. The host validates the view against that frame's signed manifest. This uses Dinah's generic workbench state argument and never accepts a page URL or another extension's identity.

Prepared terminal tickets are single use, limited to 64 outstanding entries and expire after 15 minutes. Signature and publisher checks do not certify native code safety. File decorations, workspace groups and built-in migration remain future work as recorded in the plan.

The final focus policy also accepts native focusable elements through their `tabIndex`, explicit tabindex targets and editable hosts. It rejects a plain `contenteditable="false"` display element. Browser regressions cover native summary return and that rejection. An actual child-focus message clears an abandoned excursion token; background messages cannot raise an inactive frame. The installed-build smoke harness closes its previous disposable prepared-command windows before each run.

## Package changes while hosts are running

A package change acquires an extension-specific pause before stopping its backend. The pause rejects new calls and cancels active invocation scopes, so a restored view cannot restart an executable while Windows removes it. Resuming refreshes the action and view catalog.

The CLI coordinates with running hosts through private descriptors under the extension store's `.hosts` directory. Each descriptor identifies a loopback control endpoint and a separate random bearer token. Control requests require the exact loopback Host, reject Origin, and accept only bounded pause/resume messages. Each mutator owns a distinct lease, so overlapping operations cannot release one another's pause. The CLI resumes its leases even when its operation context is cancelled. A forcibly terminated CLI can leave its lease armed until the affected host restarts; the host fails closed in that case.

Descriptors are removed on orderly host shutdown. Dead-process descriptors are ignored using documented process-liveness APIs. The descriptor grants the same local-account authority as the installation files; it is never exposed to extension frames or backend callback scopes. SDK 0.1.1 retains an installation's registry record when payload removal fails, allowing a later retry instead of hiding a partially removed package.

Native integration coverage includes CLI disable, enable, removal and reinstallation while a backend is running. Client-side MCP errors are read from failed tool-result content, since the SDK's server error accessor is documented to return nil on clients.

## PUDL 0.85.0 integration

Dinah now vendors the complete PUDL 0.85.0 distribution from tag commit `2cc2394df80d7b979861d7efca9524f23b5cb178` in both asset locations. All 143 tagged files in each location match upstream bytes, including minified assets and font variants. The upstream menubar focus API and tooltip Escape handling replace Dinah's local patches. Host and SDK excursion tokens remain responsible for restoring focus inside opaque extension frames.

The annotations module is shipped as part of the distribution but is not loaded automatically. Existing application controls and extension frames retain their explicit script dependencies.

The focus regression checks actual restoration rather than requiring entry-time rejection of an unfocusable element. Upstream accepts a visible return candidate and emits its event only if focusing that candidate succeeds. The saved-history and Document-menu fixtures wait for replay completion and menu refresh before asserting the final screen.

The final 0.85.0 run passed all 72 browser suites together in 149 seconds with four jobs. Vet passed. The full Go run hit the existing terminal-close timeout; that test passed alone in the follow-up. The isolated native host passed real Git and package-lifecycle smoke checks, and its served menubar and tooltip bytes matched the tag. One earlier disposable native instance exited during CLI reinstallation without captured diagnostics; the fresh instance with captured output passed the same sequence without stderr. This is browser and native-host evidence, with no new native WebView2 performance measurement.

The native-fullscreen integration uses the complete PUDL 0.87.0 tag and SDK 0.1.2 for host-initiated focus excursions. See [native fullscreen](native-fullscreen.md) and [exit confirmation](exit-confirmation.md) for the current host behavior.

## Additions for the Git applet's overhaul

Paul decided the five additions below on 25 October 2026; they are listed in [the plan](../plans/extensions.md) and in the Git repository's `docs/design/overhaul.md`. All five are additive within contract 1. The manifest gains two optional members, the frame SDK gains two APIs and one event, and an installed package with none of the new members behaves as it did before. The public contract text is in the SDK's `CONTRACT.md`; this section records how Dinah implements it.

### Shared state within an extension

The host keeps one map per extension ID and workspace ID in the page (`extensions.js`), in memory, while any frame of the extension is open; the map is dropped when the extension's last frame is destroyed and is never written to disk or to the Go side. A frame writes a key with the `shared-set` channel message, which the host validates (key `^[a-z0-9][-a-z0-9_.]{0,63}$`, value JSON of at most 64 KiB, at most 64 keys) and stores as a parsed copy so no frame holds a reference into another's value. The extension's other live frames in the same workspace receive a `shared` event with the key, the value and the writer's view ID; the writer receives nothing. A frame's init message includes the current map, and a following frame that moves to another workspace receives that workspace's map as `shared` events with an empty `from`. The SDK keeps a local copy for `dinah.shared.get()` and updates it from the init message, from `shared` events and from its own successful writes.

### Reusing an open view

`open-view/<id>` looks for an open frame of the same extension, view and workspace before opening a window. A pane is one per view regardless of workspace. When a frame matches, the host opens the same workbench item again, which raises the window and delivers the new state through the frame's `state` event. A payload of `{reuse: false}` skips the lookup. A window whose default key is taken by another frame of the view (a following window pinned to another workspace, or one asked for on purpose) gets a key with a random third element in its item path, which the address parser ignores.

### Following windows and the pin

A view declared `follows: true` opens with an empty workspace in its window key, as a pane does, and resolves the selected workspace on `dinah:workspace-selected`. The host adds a checkable "Pin to This Workspace" command at the head of the window's own commands; the frame cannot declare a command with the ID `pin-workspace`. Pinning keeps the workspace the window shows at that moment and stores it in the UI state under the window's key, so a restored window keeps its pin; unpinning follows the selection again. Each change, including a pin or unpin, is sent as a `workspace` event whose snapshot has a `pinned` boolean, and the window's title detail is the workspace's name through `pudlWindows.retitle`. A pane's behaviour is unchanged, and a window without the member still takes the selected workspace at opening.

### Effect levels

The SDK's `Action.Effect` is validated as one of `read`, `local`, `history`, `network` or absent, and travels in the signed manifest and in the backend's tool metadata, which the host compares with the manifest at startup as before. `Manager.Bundles` copies the level onto the applet action (`contract.Action.Effect`), and `applet.Asks` replaces the capability test in `appletConsent`: a declared `read` or `local` never asks, a declared `history` or `network` always asks, and an absent level asks when any capability is effectful, which is the rule the first release had. The invocation scope also records the level, and `host.run` asks the Show Me First review hook only when `extension.EffectAsks` is true for it, so a reviewed extension still shows each history or network command and lets reads and reversible local changes run. The Git extension 0.1.0 has no levels and keeps asking as before.

### Diffs in the Editor

`open-diff` is a host command with a payload. The host validates the payload in the page with the same bounds as `extension.DiffSpec.Validate` in the SDK, which is the authoritative Go definition and the type a backend can build the specification with. The host then calls `window.dinahDiff.open` with the specification, the key prefixed by the extension ID so two extensions cannot collide, and an `onAction(id, where)` callback. The Editor calls that callback when the person takes a file, hunk or line action; the host checks the ID against the declared actions and sends the frame a `command` event `{id: 'diff/<action id>', diff: key, where}`, with `where` copied through JSON so nothing but data crosses the channel. When `window.dinahDiff` is absent the command fails with "The Editor's diff view is not available", which is the state until the Editor's diff view lands; the browser suite stubs the view when the real one is missing and wraps it when present.

### Verification

The SDK's Go tests cover the new manifest members surviving signing, the rejection of an unknown effect and of an unknown manifest field, the effect consent rule and `DiffSpec` validation; its browser-bridge tests cover shared state and the host-command payload. Dinah's Go tests cover the effect level reaching the applet action and `applet.Asks`, and `host.run` skipping the review hook for `read` and `local` while calling it for `history`, `network` and an absent level. The browser suite `e2e/extensions-shared.js` covers shared state between a pane and a window, delivery to a newly opened frame, removal, open-view reuse and `{reuse: false}`, a following window moving with the selection while a pinned window stays, the pin command and its persistence, the second following window from another workspace, and open-diff validation, routing and the unavailable error.

## Views that open files

Paul approved on 28 October 2026 letting a view declare the media types it opens, as the request in [document-links.md](../plans/document-links.md) needed. The SDK's `View.Opens` is a list of `FileType` values, each a lower-case IANA media type and one to sixteen file extensions; `CONTRACT.md` in the SDK is the public description, and `Manifest.Validate` enforces its bounds. Only a window that is neither a pane nor following may declare it, because a file belongs to one workspace, and a manifest that declares it needs `workspace.read`, because the view reads the file through `host.read`.

The member is additive within contract 1, but it is not invisible to an older host. `ReadManifest` rejects fields it does not know, so a host built with SDK 0.1.x refuses a package that uses `opens` at installation, before anything is stored or run; it never runs such a package with the declaration ignored. A package without the member serializes and signs exactly as before. The SDK says which release a package needs through `Manifest.MinimumSDK`, which `dinah-extension sign` and `verify` print, and its `CONTRACT.md` lists the release that added each optional member. The same is true of `effect` and `follows`, which also arrived after 0.1.0. The SDK's version is 0.2.0-rc.1 in its changelog and is not yet tagged.

Dinah's side is `cmd/dinah-desktop/extension_claims.go`. `syncExtensionClaims` reads the store, skips disabled extensions and packages that fail `Manager.Installed`, and registers each declared type with the view's title, glyph and ID and the time the extension was enabled. `resetExtensionCatalog`, which every installation, update, enabling, disabling, removal and command-line change already calls, calls it again, and the resolver loads the claims the first time a link needs them. Dinah's own claims and the Editor's kinds of file come first; among extensions the one enabled first wins, by `Installation.EnabledAt`, a new store field that an update keeps and that enabling sets. The resolver builds the view's state, the JSON text of the SDK's `OpenState`, and the page opens the view with it through `openView`, the function behind `open-view`, in the file's workspace. `Extensions()` adds each extension's declared types to what Settings shows, with the file extensions that open elsewhere and the reason, which Settings draws as a warning with PUDL's warning glyph.

The SDK's tests cover the member surviving signing and breaking the signature when changed, its validation bounds, the minimum release, the open state and the enabled time across updates; its bridge tests cover `dinah.openedFile`. Dinah's `extension_claims_test.go` and the browser suite `e2e/extension-file-links.js` cover the resolver, precedence, the lifecycle and the opened view, as [document-links.md](../plans/document-links.md) lists.
