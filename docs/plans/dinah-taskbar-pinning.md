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
