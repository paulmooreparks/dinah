# Dinah taskbar pinning

Paul authorized PUDL 0.103.0 adoption and taskbar pinning, then authorized installation after verification. Both vendor folders contain all 193 files from the published archive. Its SHA-256 is eaa6e6665004e5697e21245ac21faf8ef7bdd7ba783edb77ff836e668973d97c; the release tag is 12abb6a8dbee5258d82f85baefa3e28ec707c5b5. No vendor runtime patches were made.

## Identity and eligibility

Pins represent destinations. Providers define reopening rules for applets, shells and saved items. Workbench kinds inherit support without per-item registration. The shared window template supplies explicit provider and destination metadata before adoption. Title text, icons and URL patterns do not determine identity.

Applet launcher pins use the applet kind and reopen its normal entry point. Individual editors and file browsers use their saved item identity so a pinned document or folder reopens that document or folder. Other saved items, including chat sessions, retain their workbench identity with workspace information. Shell pins use the shell identifier and start a fresh instance in the current workspace path when dormant. Pinning Default Shell selects the currently configured shell identity. Launcher creation pins and individual saved-document pins have distinct identities.

Temporary group pickers, diffs, artifact result views, failed-command views and subagent diagnostic views explicitly opt out. A kind can set pinnable to false. Unavailable kinds and shells retain saved membership but cannot create new windows until available again. Reopening uses the existing host entry points and backend permission checks. Pin metadata grants no additional authority. This is the native desktop integration; it does not introduce a browser account model.

## Persistence and activation

Versioned preferences live in the native-profile UI-state entry dinah-taskbar-pins-v1. Pins, combined order and provider references are saved together. Workspace identities remain inside saved-item addresses. This profile-wide scope matches Dinah's cross-workspace desktop. Preferences are separate from the shared window-layout URL.

Provider resolution never opens a window. Unopened launcher rows resolve only when their shared action menu is requested. Ordinary destination activation uses PUDL's waitUntil contract and waits for the host applet readiness promise. Failed window adoption or applet initialization rejects activation and reports through the existing error UI. Running pins use PUDL's existing restore and stack behavior. Unpin does not close windows.

The launcher retains its closed, unpinned first-use default. Launcher-pane pinning is independent of taskbar pins. Pin and Close remain right aligned in the header, and Size is right aligned in the footer.

## Verification

Isolated browser tests cover unopened launcher pinning without opening a window, dormant activation, close retention, two items from one category without registrations, explicit exclusions, individual document and folder identity, destination pictures, readiness failure settlement, native preference snapshots, reload restoration and unavailable-provider retention. The existing workbench, terminal-shell and window-stack checks cover adjacent behavior. Tests use a stubbed native bridge and do not establish a native restart or manual assistive-technology result. Installed identity and rollback evidence are recorded separately when installation completes.

## Installation

Build 0.0.0-taskbar-pins-20261107 was installed under Paul's authorization. Its SHA-256 is 881A2546D9DC99A36FFA991A06F4300121D41FEA3B452904B228403255577A71. The previous executable is retained at C:\Users\paul\.dinah\bin\dinah-desktop.exe.before-taskbar-pins-20261107-050935. The receipt is build/taskbar-pins-install/install-receipt.json. The desktop was not restarted. Focused pinning, terminal, launcher, workbench and window-stack checks passed; native restart verification remains with Paul. Extension launcher IDs are encoded from their explicit registry identities, so dotted manifest IDs remain valid pin keys. Disabled or removed extension launchers resolve unavailable.

## Terminal exit behavior

Paul requested closing terminal windows when their underlying application exits. Successful native-shell exits now close the associated window. Nonzero exits and startup failures retain output and the existing restart action so diagnostics remain readable. Closing is deferred until startup event replay finishes, and the callback checks that the same session remains ended. A successful exit does not send a redundant termination request. Taskbar pin membership remains intact after its last terminal closes. The terminal-shell browser suite passes normal-exit, nonzero-exit/restart, early-exit and retained-pin checks in build/terminal-exit. This follow-up source change has not been installed.

The terminal-exit follow-up was installed under Paul's authorization as 0.0.0-terminal-exit-20261107. Installed SHA-256: 934CB2F51C3009207FB3CC58BC60E7CA68274930960F24D54DA32CE5BB84FC92. Rollback executable: C:\Users\paul\.dinah\bin\dinah-desktop.exe.before-terminal-exit-20261107-052854. Receipt: build/terminal-exit/install-receipt.json. No restart was performed.

## Taskbar label truncation

Taskbar buttons display the window title followed by the dot separator and workspace as one ellipsized text node. The workspace has no reserved width. Full labels remain in hover tooltips. Window headers and stack rows retain their existing layout. This is a local patch to the bundled PUDL window renderer, including both distribution copies and regenerated minified assets; it must be reconciled on the next PUDL update.


## Host-owned combined titles supersede the renderer patch

Dinah now combines title and workspace/detail before calling PUDL's existing retitle API, passing an empty separate detail. Saved title components remain separate in Dinah's state so restoration does not duplicate the suffix. Pin descriptors receive the same combined label and refresh when titles change. Window headings also receive the combined title. PUDL retains responsibility for shortening and folding. Both bundled window scripts and their minified copies/maps have been restored from the verified 0.103.0 archive. Focused taskbar label, pinning and terminal checks passed. This change is not installed.


The combined-title build was installed on 2026-11-07T12:35:49.3697007+08:00. The installed executable matches SHA256 D0E95E92B3B005C4C32E3247FA13FEBF9FCA329EAEF7A9E5954160EDB3F2E6F5. Rollback executable: C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-combined-titles-20261107-123549. Dinah was left running for the person to restart.


## Candidate branch prepared

The existing installed work was committed as 902df02 and fast-forwarded into master before creating feature/pudl-pinned-section-candidate. The frozen pinned-section-73c8338a72e3 distribution is now adopted in both bundles. Provenance and hash checks are recorded in the frontend PUDL.md. Host adaptation and regression execution are paused pending the person's next instructions. The installed executable and native profile remain unchanged.

## Frozen candidate host regression, 7 November 2026

The isolated headless Edge harness executed against host commit 8586abe on feature/pudl-pinned-section-candidate and candidate pinned-section-73c8338a72e3. It serves local frontend assets through the test route and stubs the native bridge. Each browser uses a fresh context; persistence tests use fixture session storage. The installed executable and real profile were not used or changed.

The runtime frontend manifest is retained at build/pinned-section-regression/host-manifest.json. Its SHA256 is 7A5DD7BF9F71874A749B6BDE13A1F7EF5A9B3D89C15DFF00F96649EC921E4F17. The candidate archive SHA256 is e8d7bf70b15416338769b1aa12183ea98dd2baffb5d4929da2775c26662f00c0, with source fingerprint 73c8338a72e3c00f82b6a7b47804ed0fa4808e2625d05ed09bca0d85424b703d.

All six suites passed in their final runs. The existing taskbar-pins, taskbar-labels, title-launcher and launcher-commands suites passed. Two added candidate suites produced 28 passing checks, including two page-error checks, with zero final failures. They cover singleton minimize/raise/restore without duplicate entries, folded heading and stable button height at 390px, menu pin/unpin and restored provider references, applet context suppression, launcher exclusion, pointer/keyboard Arrange toggling, order draft cancellation, stack chooser behavior and running/dormant pictures. Existing suites cover unavailable references, combined title truncation, readiness failures and launcher behavior.

Evidence is in build/pinned-section-regression. The candidate-corrected.log and stacks.log retain all added checks. Screenshots include candidate-wide.png, candidate-narrow.png, candidate-restored.png, candidate-dormant-picture.png and taskbar-combined-label.png. The narrow screenshot was inspected. The initial candidate.log retains two failed assertions caused by using immediate Unpin as an order-draft mutation; the corrected test uses Move later. Those were test errors, not candidate defects.

No production code changes were required during this verification. Tests and this record remain uncommitted. No desktop build was installed or restarted. These results do not claim native restart, manual assistive technology, real touch devices, Firefox or WebKit consumer testing.

Paul subsequently authorized installation. The tested unpublished candidate was built and installed on 2026-11-07T14:59:09.1303573+08:00. The installed executable matches SHA256 848F2C0DE32C2E8F076A2C07FF0BA7D7048C315AFB8A9F72EBD674EE72F1B8D4. Rollback executable: C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-pinned-section-20261107-145908. Dinah was left running for the person to restart. The installation does not publish the upstream candidate.


## Published 0.104.0 installation

The complete published distribution replaced the candidate after archive, tag and copied-file verification. Only pudl.css changed from the candidate. The pinned-section, stack and combined-label suites passed. The build was installed on 2026-11-07T15:31:27.7912672+08:00, with SHA256 B1957E3643F0366CCF1906AD428E491F4FA9FB2D6E8D1EB4D0A39EBFE1D0DD7A. Rollback executable: C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-pudl-01040-20261107-153127. Dinah was left running for the person to restart. Receipt: build/pudl-01040-release/install-receipt.json.


## Published 0.105.0 adoption, 9 November 2026

Both bundles now contain all 193 distribution files from the latest published release, v0.105.0. GitHub's latest-release response and tag reference were checked directly. The tag commit is e329c43c51c4a19df35181f5a9421219fd48c864. The downloaded 7,186,020-byte archive matches the published checksum, 5c0e4f7ed32a3ca41e13e4c393ef74b7029b214d0aca4c7eaef57045fc98081d. Each copied file was hash-verified in both bundles, with no extra runtime files. VERSION is host metadata set to 0.105.0.

Six isolated headless Edge suites passed: taskbar-pins, taskbar-labels, title-launcher, launcher-commands, pinned-section-candidate and pinned-section-stacks. Evidence is in build/pudl-01050-release/browser-final. An initial invocation named a nonexistent terminal-shell suite and ran no tests. The subsequent invocation failed because playwright-core was absent; npm ci restored the existing locked dependencies and all six suites passed. The original dependency failures remain in build/pudl-01050-release/browser. No host runtime edits or vendor patches were required.

This adoption is a working-tree update. It does not install or restart the desktop, merge or commit the existing work. The installed executable remains the previously verified background-replay build. Native Restart validation remains outstanding.

The production build 0.0.0-pudl-01050-20261109 passed. Its SHA256 is A5C0FE1489E0CAB1EDAEDFA14564D060A0F92D0FAD1556B8E46409D5A0F3F6F8. The executable and results.json are retained in build/pudl-01050-release. It has not been installed.

## Current status after commit, merge and installation

On 9 November 2026, commit 8315d4f integrated PUDL 0.105.0, the chat restoration and provider shutdown fixes, regression tests and this handoff. It was fast-forwarded into master in C:/Users/paul/source/repos/studio. The outstanding source changes described below are now committed. The earlier sections remain as historical investigation context.

Build 0.0.0-pudl-01050-20261109 was installed at 00:38 Singapore. Its SHA256 is A5C0FE1489E0CAB1EDAEDFA14564D060A0F92D0FAD1556B8E46409D5A0F3F6F8. The rollback executable is C:/Users/paul/.dinah/bin/dinah-desktop.exe.before-pudl-01050-20261109-003840. The receipt is in the install worktree at build/pudl-01050-release/install-receipt.json. Running process IDs remained unchanged; the desktop was not restarted. Native Restart validation remains outstanding.

All six launcher/taskbar suites passed with 0.105.0. All seven replay suites also passed against the new distribution before merge. The production build passed. Private transcripts, build artifacts and unrelated main-worktree files were excluded from the commit. Nothing was pushed.

The outside agent can now begin from master in the main studio directory. Private evidence remains in C:/Users/paul/source/repos/dinah-install-20261105/build/. The preserved cleanup/history-capture stash remains untouched.
