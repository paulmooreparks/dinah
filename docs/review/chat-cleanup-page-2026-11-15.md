# Clean Up Chats page review

The page is implemented on `frontend/chat-cleanup`, stacked on Engine's contract commit `87d96344`. It consumes the amended five-call contract in `docs/design/chat-cleanup.md`. Engine owns the host implementation and Quality owns its independent acceptance. This record covers browser presentation, not native host acceptance.

## Result

Clean Up Chats opens as an addressable workbench page from a visible button above the Chats list, the Chats browser menu, the chat's Actions menu and View. The command also appears in the generated keyboard help. Workspace, agent program, age and closed-only filters are stored in the URL. The page uses the host's oldest-first snapshot, eligibility and refusal text. It displays unknown dates and counts explicitly and includes earlier agent programs when filtering switched chats.

Selection and batch outcomes leave each existing row in place. A deleted row remains as a result until Refresh. The Recycle Bin has its own scrolling region with Restore, Delete Now and Empty Recycle Bin. Final confirmations use the host's row or aggregate text verbatim. The page displays remaining agent-copy explanations, including a successful local deletion whose agent program could not delete its copy. Loaded-program notes remain visible in the bin.

Every single-chat entry now says Delete Chat and calls RemoveChat. The previous provider-dependent Hide label, DeleteSession call, HideSession fallback and Show Hidden Chats Again command are removed from the page. Recovery cards use the same confirmation and operation. Staging preserves the unsent chat message; final deletion forgets its composer value. Document drafts and referenced files are untouched.

Removal events discard cached rows and controllers without refreshing provider history. Restore puts the authoritative result into the closed-chat metadata cache. Neither path calls Sessions, SessionsAt or OpenSessionAt. This avoids restarting a program merely to update the list.

## Verification

The final source passed `app-ui3`, `chat-cleanup`, `drafts`, `menus-through-updates` and `recovery-card` together through FIFO in 54 seconds, exit zero, under `%TEMP%/chat-cleanup-final-20261115`. The two bin-error assertions passed after failing before the result-routing correction. The final fixture-only run sorted the invented snapshot according to the host contract and passed in 5 seconds, exit zero, under `%TEMP%/chat-cleanup-contract-final-20261115`, ticket 2121. The attached pictures and frame samples come from that run.

Focused desktop checks for frontend ownership, asset exclusion and UTF-8 passed in 0.440 seconds through FIFO, ticket 2118. JavaScript syntax and whitespace checks passed.

The frame fixture samples the toolbar, taskbar and every existing row immediately before the batch completes and in ten following animation frames. It also checks selection and the delayed-operation state, confirms cancellation changes nothing, verifies independent failures, checks both final-deletion confirmations, preserves composer text until final deletion, restores URL filters after reload and measures the narrow layout. Its data is invented. Screenshots and frame samples are attached beside this note.

The new refused-Restore and failed-final-deletion assertions first failed in `%TEMP%/chat-cleanup-bin-refusal-before-20261115`. The page had chosen its result target from the returned status, which cannot identify the source list on a refusal. The correction records whether an operation came from Chats or the Recycle Bin. The assertions remain unchanged after that correction.

Earlier fixture corrections scoped Restore to row actions instead of the window's Restore button and scoped confirmation paragraphs to the actual HTML dialog instead of every workbench window with a dialog role. Explicit select labels corrected an accessibility issue found while exercising filters. The single-chat suites were updated for the requested recycle-bin behavior and continue to refuse provider deletion during staging.

The broader `workbench` suite fails its existing assertion named "Tools launchers have one shared searchable catalog, with the Editor". The same unchanged assertion fails on detached pre-cleanup master `c4690b24` in `%TEMP%/chat-cleanup-workbench-baseline-20261115`; it also failed in the combined page run. No workbench assertion was changed.

## Limits and requested review

The browser checks use the real frontend with an invented host fixture. They do not prove retention, expiry, native provider deletion, lease revocation or process disposal. Engine's implementation and Quality's independent cases must supply those results on the combined candidate before acceptance. The browser fixture does prove that the page makes no provider-discovery call when staging or restoring.

No real chat was deleted. No real home was written, no credentials were read, and no visible process, push, merge or installation was used. Engine and Team Lead should combine this immutable UI candidate with the host implementation and review their evidence together.
