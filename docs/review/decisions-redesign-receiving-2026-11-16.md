# Decisions redesign receiving

The approved Decisions page is implemented on frontend/decisions-build from eaf3c402. The branch merges mockup 64c0b722 and the earlier task-question presentation branch. The separate workbench fixture repair is 6a74d71f.

The page has an oldest-first list, fixed request controls, independently scrolling About pane and addressable narrow panes. Each row has the same keyboard and pointer menu, including disabled commands with reasons. Finished requests leave the list and the next request opens. Radio choices retain an independent own-answer draft across navigation and reload. Draft saves keep Send and its status stable; submission still flushes the latest draft and retains the existing receipt/reconciliation protocol.

Notifications and Decisions share task answers and their pending gate. Typed text preserves over-limit input and counts Unicode scalars. Multiple selection preserves the offered order. Legacy questions retain their original entry point. Task questions create no chat identity or recommendation.

## Verification

The 25-suite browser regression passed in 33 seconds. It covers Decisions, task questions, Notifications, chat approvals, shared inventory and workbench layout. A five-suite follow-up passed in 28 seconds after making read failures visible in the fixed footer and empty state. The final four-suite selection passed in 15 seconds after preserving own text independently of a selected suggestion. The final selection includes 78 rendered checks across light/dark at 1440/390, plus draft reload and malformed-state checks. All runs used the shared queue, headless browsers and invented bridge fixtures. There were no browser errors.

The original layout failures remain private. The corrected tests preserve request identity, revision, explicit-send, lost-receipt, stale-response, pagination, literal context and draft assertions. Their presentation bindings now follow the approved footer, radio and menu contracts. The old dismissal interaction checks explicitly require disabled controls and zero dismissal calls pending the new host.

The neutral public mockup passes all 69 walkthrough checks. Its 132 captures report no overflow or page errors. Original demo captures and overview images remain private. The public examples and matching checks were neutralized to satisfy the private-term guard; their interactions are unchanged. Four product screenshots are in the adjacent decisions-redesign-2026-11-16 folder. Raw compiler output is excluded.

## Remaining host boundary

Dismiss is visible disabled with an explanation. The person's menu has no Withdraw. Engine confirmed in message 9906 that the current task end event cannot distinguish an answer from cancellation. Until its settlement event and passive record land, the page says only that a question is no longer waiting. No disposition is inferred from disappearance.

This receipt covers producer browser verification. It excludes independent Quality clearance, native installed-window acceptance and a separately built fork. The final package-policy and public-guard receipts accompany the handoff. Lead receives the reviewable branch; Front End retains presentation corrections and the later settlement binding. Nothing was pushed or installed by this task.

## Publication provenance

The package asset and frontend-policy selection passed in 0.367 seconds. The profile, private-term, commit-message, branch-diff and local-path guards passed on the integration source. The final handover also supplies the guard receipt for the public delivery tip.

The local frontend/decisions-build branch retains the approved mockup merge for review. Its original mockup ancestors contain the excluded demo labels. The public frontend/decisions-delivery branch carries the sanitized final tree on top of the standalone fixture repair, so publication does not introduce those original mockup blobs into history. The two delivery trees are identical. The final note and whitespace cleanup do not change the tested product code.

## Independent receiving corrections

Quality reported three page defects on the first delivery. PUDL documents its data-row-actions marker as a pointer-only affordance, and its initializer removes the marked button from the Tab order and accessibility tree. The page now uses its own data-decision-actions marker for an ordinary accessible button, while keeping the existing PUDL menu and row keyboard commands. The vendor files are unchanged.

Opening the phone About pane now focuses Back to Request. Returning focuses About This Request. The empty state includes the approved explanation of which requests appear, and its separate title still carries a read error when necessary.

The producer matrix adds actual Tab navigation, accessibility attributes, both focus transitions and the empty explanation at both widths and in both themes. Its first queued invocation was cancelled before browser startup to complete the empty-state fixture. The replacement selection is queued behind another owner's run; no pass is claimed yet. Quality's original failures remain in its independent evidence. Front End retains these corrections until receiving closes them.

Quality subsequently found that the completion confirmation lost its destination when the next request opened. The send operation now retains its display destination alongside its existing receipt metadata, without adding fields to the host request. Confirmed completion names that destination. Older saved operations use the matching request's destination when available. The saved-draft reader validates the optional display string. The producer check asserts the previous asking chat and group remain named after auto-next. The second producer queue and guard were cancelled before starting to include this correction.

The eight-suite producer selection passed in 17 seconds with no browser errors. It covered Decisions redesign, errors, list, narrow layout, reading, drafts, delivery and races. All four layout/theme combinations passed actual Tab access, phone focus, empty explanation and destination confirmation. A final source review moved receipt validation ahead of the destination lookup so malformed responses still take the existing confirmed-result error path. The unchanged selection is queued again against that final source. Raw output remains private.

Quality found a further sequence where a new request's failed draft save was overwritten by the preceding completion sentence. The current draft error now has final precedence in the fixed footer, retaining the unsaved input and disabled Send. The matrix reproduces completion, auto-next, a rejected SaveUIState and a successful later save in all four layouts. The queued final validation and guard were cancelled before starting to include this sequence.

The final eight-suite selection passed in 18 seconds on product source 611c15fa, with no browser errors. This includes the rejected-save sequence after completion in light and dark at 1440 and 390 pixels. The prior passing selection's wide-light and phone-dark screenshots were inspected and fit their reading surfaces. The final profile and private-term guard receipt accompanies this documentation-only child. Independent Quality acceptance remains separate and pending. No runner is needed beyond the final guard; nothing was pushed or installed.

## List-read notice

Quality found that a failed read with the list open and no request selected was announced only through the clipped live region. The list pane now has a visible PUDL danger notice with the host error and Refresh. The last good rows remain in place. A successful read clears the notice, and the existing detail-footer and empty-state error paths remain. The producer matrix now rejects Decisions with no selection, checks the notice's actual nonzero bounds and preserved rows, then retries through its visible Refresh control in all four layouts. Verification is queued; this source handoff does not claim a pass.

The five-suite producer selection passed in 33 seconds on source 38d160bf, with zero browser errors. Decisions redesign, errors, list, narrow and reading all passed. The new list-notice failure/retry assertions passed in light and dark at 1440 and 390 pixels. The profile, private-term, commit-message, branch-diff and local-path guards passed on that source. This documentation-only child records the receipt; final text scans cover the child. Independent Quality receiving remains separate. No producer runner remains active, and nothing was pushed or installed.

## Final public mockup scrub

Lead found remaining private workspace and customer-document labels in the mockup after the official guard had passed. Those labels now use invented Harbor names in both the clickable mockup and the current-page fixture. All 132 mockup pictures were regenerated from the neutral source with no reported overflow or page errors, and all 69 walkthrough checks pass. Product files remain byte-for-byte identical to the independently cleared 336eb1e7 source.

The replacement public delivery is a single final-tree commit above the existing clean fixture-repair base 6a74d71f. The rejected delivery tips and their mockup blobs are excluded from its ancestry. This replaces the earlier delivery provenance. The original integration and rejected delivery branches stay local and must not be published. The added private pre-handover scan checks every newly reachable text blob and commit above the clean base for the supplementary labels, in addition to the official guard and private-term checks. Binary pictures are regenerated evidence, rather than searchable source text. Quality should confirm its product-file clearance carries to this text-only scrub before Lead receives it.

Lead found one additional task title using the same customer naming convention. It now reads "Refresh the Harbor architecture diagrams". All 132 pictures were regenerated again, with no reported page errors or overflow. The supplementary history scan now rejects the customer prefix followed by any number, rather than a single document number. The final neutral delivery again contains one commit above 6a74d71f; the earlier neutral candidate is excluded from its ancestry. Product files still match the independently cleared source, so this correction changes mockup text and captures only.
