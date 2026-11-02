# Invariants

Each line below is a rule the November 2026 review established, followed by the guard that fails if the rule is broken. A reviewer checks a change against this list. `pwsh scripts/guards.ps1` runs every automatic guard listed in `scripts/guards.json`, and the CI build depends on the same command, so a guard that is deleted or skipped fails the run. The findings behind these rules, with their fixes, are in `docs/review/ledger.md`.

## Automatic

1. No tracked file contains a term from the private forbidden-terms list, and the exact tip is checked before every push. Guard: `TestNoForbiddenTermsInTrackedFiles`, which runs locally with `DINAH_FORBIDDEN_TERMS` set. CI has no copy of the list, so there it must report itself skipped.
2. The product's name appears only in `internal/profile`. Guard: `TestNoProductNameOutsideTheProfile`.
3. A provider's name appears only in its provider row or in that provider's own files. Guard: `TestNoProviderNamesInSharedCodeBeyondTheList`.
4. When an agent reads or writes a file through Dinah, protection is decided by the identity of the opened file and its folders, never by how the path is spelled, and every Windows path that is not a plain drive-letter path is refused. Guard: `internal/agents/path_identity_test.go` and `path_identity_windows_test.go`.
5. A file read by an agent returns at most 1 MiB of a validated line range and never panics. Guard: `TestSliceLinesRejectsInvalidRanges`, `FuzzSliceLines`, `TestProviderReadPolicyAndBounds` and `FuzzReverseRequestParameters`.
6. A fault while handling one provider request or notification ends only that message, never the connection or the process. Guard: `TestReverseRequestPanicKeepsConnectionServingOtherChats` and `TestNotificationPanicKeepsConnectionServingChats`.
7. Every connection diagnostic passes through the one redacting sink. Guard: `TestDiagnosticsCannotBypassConnectionRedaction` and `TestMalformedProtocolCannotPersistCapabilities`.
8. A browser sign-in is stored only as a SHA-256 digest, expires after 30 days idle or 90 days in all, and can be revoked for one browser or for all of them. Guard: the `TestWeb…` tests in `internal/bus` and `TestBrowserSignoutScopeAndSignInAgainPage`.
9. Content from agents and documents paints only inside its own box. It cannot add a style element, a positioning or stacking style, an element in the top layer, or an id or name that reaches the whole page. Guard: the `contained-content` and `html-safety` browser suites.
10. Text from anyone other than Dinah reaches the Terminal only through its inert filter. Guard: the `terminal-escapes` browser suite.
11. The page has one HTML escaper, `dinahHTML.text`, and a selector built from data always goes through `CSS.escape`. Guard: `TestFrontendJobsStayInTheirHomes` and `TestFrontendRulesFindTheirHomes`.
12. A saved address cannot take the bus pages off their origin, and an extension's view glyph is checked as strictly as its command glyphs before it reaches a style. Guard: the `web-skins` and `extensions` browser suites.
13. A native extension inherits only the documented standard environment and the variables its signed manifest declares, and installing one says that it runs with the person's full local authority until WASM isolation lands. Guard: `TestExtensionEnvironmentUsesAllowListAndSignedDeclarations` and `TestExtensionEnvironmentSupportsGitCommands`.
14. No published combination of operating system and architecture has a called vulnerability. Everything is built and scanned with the Go version on go.mod's `go` line, and the installer refuses binaries built with any other. Guard: `tools/vulnguard`, `TestEveryPublishedPlatformHasABlockingSourceScan`, `TestModuleRequiresItsSelectedCompiler` and `TestInstallRefusesWrongCompilerBeforeReplacingEitherProgram`.
15. Every CI action is pinned to a commit, building waits for the vulnerability scans, the guards on Windows and Linux and the full Go and browser suites, and publication downloads only the build archives. Guard: `TestEveryPublishedPlatformHasABlockingSourceScan`.
16. A definite refusal from a provider settles a continuation record, and an uncertain outcome blocks a blind retry. Guard: `TestAFailedCompactionPutsTheHoldAndTheQueueBack`, `TestRejectedAllocationSettlementIsTransactionalAndStageChecked` and `TestUncertainCompactionAllocationBlocksBlindRetryAcrossRestart`.
17. Nothing moves, resizes or loses focus when live state changes. Guard: the `list-state-stability`, `taskbar-state-stability`, `list-row-layout` and `chat-last-read` browser suites.
18. Every chip, row and title reads a chat's state from one projection. Guard: the `chat-state-contract` browser suite.
19. Repository text checks out with LF on every platform, and vendored release assets retain their supplied bytes under every Git line-ending setting. Both hosts validate admitted skin digests before running. Guard: `TestBundledSkinBytesSurviveWindowsCheckout`.
20. Dinah styles or scripts nothing inside PUDL's own markup; a capability PUDL lacks is requested upstream, and Dinah waits for the release. Guard: pending, a static check from FE-32.
21. Every agent read that takes a path goes through the deny policy and is written to the audit log, and the reads that take no path are the exhaustive set of four: Skills list, Skills read, prompt-history and read_attachment. Guards: `TestPathlessSkillReadsRejectPathsTraversalAndSymlinksAndAudit`, `TestPathlessHistoryRejectsSelectorsAndSymlinkAndAudits`, `TestAttachmentPolicyRefusesDeniedSendAndLaterRead, TestAttachmentPolicyRejectsSymlinkEscapeAndKeepsLiveChanges and TestAttachmentPolicyStoredCopiesStayInCatalogueAndAudit`, and the file-access policy suites.

## Process (no automatic guard)

22. No design branches on, or argues its correctness from, undocumented behaviour of an external system. When the documented capability is missing, that is reported and decided, not worked around.
23. A ledger finding closes only when its fix and its guard are both on master.
24. No agent takes screenshots of the person's desktop or drives it. A missing applet action is reported as a capability gap.

## Known limits

The page is browser-tested on Windows only, with Edge. Linux and macOS builds are vulnerability-scanned and compiled, but their pages are not exercised.

CI runs every guard on Windows and runs the Go guards again on Linux, so a guard that only means something on Unix, such as `TestProtectedIdentityCreationModes` for the permission bits of files and folders an agent creates, runs there. No guard runs on macOS. `scripts/guards.json` lists each platform-specific guard with the systems it runs on, and elsewhere the runner requires it to skip.
