# Retained development context from the local archive

This summary was reviewed on 9 November 2026 while clearing historical untracked files from the main checkout. It preserves useful decisions and investigation leads without copying private incident data or treating historical findings as current defects. The application source and installed executable were not changed by this cleanup.

## Archive and recovery

The private archive is under `%LOCALAPPDATA%/DinahPrivate/untracked-archives/studio-20261109-010823/`. It contains 276 files under `files/`, preserving their former repository-relative paths. `manifest.json` records the source paths, sizes and SHA256 hashes. Every destination hash was checked after moving the file. The archive includes the Clear Chime preview, research material, review records, screenshots, a crash dump and local Devin hooks.

The existing cleanup/history-capture stash and ignored diagnostics are separate. They were not moved or deleted. The pre-existing .gitignore edit was left untouched. The old nested taskbar worktree and branch had already been removed under separate authorization after checking the incorporated behavior.

Restoring a file requires checking for a newer file at its original path first. Raw security reports, support drafts and historical release records remain private pending a separate content review. The archive itself must not be staged or published.

## Performance investigation lessons

The archived responsiveness note records a rejected scheduler prototype. In its historical paired measurements, some input measurements improved while overall style/layout work worsened. Preserve that negative result when considering another scheduler change. It does not establish the performance of the current build.

A subsequent stylesheet experiment reported improved steady-state style/layout costs but a regression during window closing. Those figures apply to the specific historical candidate and workload. Do not reapply its vendor patch to the current PUDL release without checking upstream changes and measuring the current host.

The useful investigation procedure is to capture a controlled reproduction, distinguish script time from style/layout time, and compare typing, window transitions, reopening and background activity. Retain unfavorable measurements and reading-position correctness alongside successful results. Aggregate timings alone do not identify the cause of an intermittent stall.

The original note also proposes event batching, DOM retention changes, terminal flow control and worker use. These are investigation ideas, not approved implementation tasks. Verify current code and documented dependencies before relying on its old line references or architectural descriptions. Current work should begin with the maintained responsiveness plans and the external-development handoff.

## Members dropdown decisions

The approved team-menu document records shared presentation for Team and Team Lead chats. The current frontend still has the Members implementation in `cmd/dinah-desktop/frontend/bus-applets.js`, with focused suites named `members-dropdown`, `member-rows` and `member-stability`. This cleanup checked their presence but did not rerun them or establish full acceptance.

The retained interaction decisions are:

- The dropdown lists chat members, and its count excludes the human participant without changing group membership or routing.
- Clicking a member opens that chat. Team rows retain their separate action menu; Team Lead rows show direct reports without that extra menu.
- Address Message inserts the bound mention at the saved composer caret, preserves surrounding draft text and restores focus.
- Last Read and Members retain their toolbar priority as the shared toolbar folds.
- Background updates preserve open-menu selection, keyboard focus and command identity.

The archived mockup is a historical visual reference. It is not a routing implementation, and its relative PUDL links may need adjustment if opened outside the old checkout location. Do not treat the archived ownership assignments as current work orders.

## Security and release follow-up

The private review ledger and supporting reports are useful audit evidence. Their findings, commit IDs and release statements belong to earlier snapshots. A new owner should reconcile each relevant finding against current code, tests and published evidence before calling it open or closed.

The history-rewrite completion record and prepared support request address different outcomes. A rewritten public branch does not establish removal of cached objects or third-party copies. The archive records support follow-up as unconfirmed at that time. Verify its current disposition with the repository owner before taking external action. Do not copy confidential historical examples into tracked documentation.

The parked Save evaluation records a gap between isolated test success and native completion verification. It also records that a submitted operation with an unconfirmed outcome had no demonstrated recovery protocol. That is a historical investigation lead, not a current defect claim. Recheck the present implementation and authorization before resuming the parked worktree or running a native test.

## Material retained only for history

Old release receipts, overnight assignments and review queues remain in the archive for provenance. They are not current execution instructions. The sodium-ion research and its downloaded sources are unrelated to Dinah. Pasted screenshots and the crash dump have been preserved without adding them to application documentation. Devin hooks are local integration configuration; restore them only when that integration is required.

The current code and installed-build handoff is [the external development handoff](../dinah-external-development-handoff-20261109.md). This summary adds historical context and does not supersede its build identity or remaining verification limits.
