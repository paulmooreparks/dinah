# Resume failure presentation and appearance follow-up

Front End owns this correction from released 40d133aeb8e2d4181d75aa89c0f225ef275f43fc, assigned in bus 2408. Engine separately owns the native continuation/persistence investigation. The frontend change does not establish why a provider thread cannot resume and does not reopen or replace native sessions.

## Failed opening

The retained saved-copy reconnect branch in session-lifecycle.js cleared s.loading, restored the saved log and returned after stateLine/sessionNote, skipping the later drawLoadingChip call. The visible failure and recovery panel could therefore coexist with an obsolete Loading status. The correction redraws the loading chip in that branch after loading stops, preserving the log, metadata and failure controls.

The initial visibility-only regression passed because compact PUDL headers can hide the whole status group. A stricter check of the status element's hidden property reproduced the defect in provider-open and current-history failure cases. Both original results remain preserved under .diagnostics/resume-appearance-20261101. The existing loading lifecycle matrix covers delayed opening, retained saved content, title/team metadata, close/reopen, retry and journal generation boundaries. The strengthened regression checks the terminal failure before retry so a successful retry cannot conceal stale state.

## Unknown and the Skin menu

Unknown replaces the visible State unavailable placeholder. The missing execution-state interpretation, muted presentation, empty glyph and accessible explanation remain unchanged. Lifecycle and group aggregate meanings stay intact.

View > Skin uses the admitted labels from dinahAppearance.snapshot().skins and invokes dinahAppearance.selectSkin, the same transaction used by Settings. It adds no preference store or private list. The checked row follows the saved preference. An unknown saved ID remains a checked, disabled unavailable row while Standard is effective. Failed saves surface through the existing menu error channel and leave the previous preference and draft intact. Stylesheet failure retains the checked saved choice and existing fallback explanation.

## Published dependency boundary

The redundant PUDL title execution marker cannot yet be suppressed through an admitted API. Both bundled VERSION files and GitHub's latest release remain 0.95.2. Release's fresh verification and PUDL-owner response are recorded in bus 2412, 2419 and 2423. The owner is preparing a reversible candidate, but the proposed data-win-state-title="none" attribute is not implemented or published. Final publication approval and immutable release evidence remain pending. This candidate does not set that attribute, modify bundled PUDL assets, remove status pills or hide generated markup through selectors. Static media glyph adoption remains on the same dependency boundary.

## Verification boundary

Focused output and screenshot records are retained in .diagnostics/resume-appearance-20261101. The strengthened twelve-mode loading matrix, shared state contract and existing state-surface suite passed in the corrected run. The new ordinary/group Skin menu matrix passed separately in 9 seconds after correcting its unavailable-row matcher to include the accessible explanation and waiting for authoritative menu refresh. Its sixteen skin/mode/motion combinations preserve mounted content and drafts, retain Unknown semantics and cover persistence failure, unknown preference and disabled stylesheet fallback. The first matcher failure is retained. Fallback-read and no-saved-copy opening failures passed in 4 seconds, including pending-loading visibility, known title, terminal error/retry controls and available saved-history recovery. Representative screenshots were inspected; no full successor gate is claimed. The candidate handoff records final outcomes and exact identities. Quality owns independent ordinary/group review across admitted skins, both modes and motion preferences. Release owns merged-master gates and installation. The original reported PUDL chat failures, saved-history recovery and installed resume acceptance remain open for Engine and Quality. No running app restart is authorized by this frontend correction.
