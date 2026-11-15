# Chat cleanup implementation record

The host work is on `engine/chat-cleanup` in `.worktrees/engine-chat-cleanup`. The contract is `docs/design/chat-cleanup.md`. This branch is incomplete and is not an implementation handover.

## Completed foundations

The bus can retain a recipient's queue while refusing new messages, delivery and wakeup. Restore reactivates the address with its old deliveries dormant until explicit registration or accepted new input. Ordinary group and Decisions views exclude binned recipients. Final queue disposal remains distinct from retained removal.

The cleanup DTO and local listing read saved messages without starting a provider. The temporary chain helper includes bindings, current attempts and archived attempts. Release and Engine agreed the membership boundary in bus messages 8552, 8572 and 8575. The interim implementation is logical_chat_members.go. Listing consumes that resolver, which excludes distinct recipients and rejects cyclic or ambiguous committed ownership. Unknown allocation outcomes carry no invented native identity. The durable model design record remains Release's responsibility. Transcript ancestry alone does not confer ownership. Cleanup will consume Release's single logical-membership function after its concrete types are agreed.

The archive helper moves chat-owned files outside startup scan folders and resumes interrupted moves. Restore refuses conflicting active files without overwriting either copy. Its settings payload has a narrow type which cannot serialize workspace records, templates or credentials. UI-state saves preserve retained composer drafts and filter permanent tombstones under the same mutex that finalization must hold.

## Verification

The full bus package passed through FIFO after the retained-recipient changes. Focused desktop tests passed for chain membership, transcript-only fork exclusion, local closed-chat counts, archive moves, interrupted resume, conflicting restores, settings ownership and stale composer saves. The existing UI-state tests passed with the stale-save filter. The first broad run found the missing ListChatCleanup entry in pageMethods. That entry was added as a query. The updated tree then passed the complete desktop package in 179.287 seconds and sessionhost in 20.748 seconds through FIFO. These passes verify the foundation only; the next lifecycle handover requires a full desktop run from its final tree.

## Remaining work

The lifecycle now stages whole logical chains, rechecks membership generations and eligibility, disposes local runtimes and revokes applet authority before retaining chat files outside startup scans. Restore, expiry and final deletion share durable recovery phases. Final deletion attempts documented provider deletion oldest first, retains truthful remaining-copy results, erases local chat records and composer drafts, and redacts chat-owned migration-backup entries. Compatibility methods stage removal, and the page methods are classified. The tagged acceptance observer reads persisted settings, retained files and bus state. Combined drawing-lease revocation, Release's durable-model adapter and independent acceptance execution remain required before handover.

Release owns the durable logical-chat model and its resolver. Engine owns cleanup lifecycle, archive and host DTO files. Front End owns presentation files and fixtures. No push, merge, installation, remote write or real-home operation has been performed for this cleanup work.


The interim membership and local listing passed focused cleanup tests after the full foundation run. Those later resolver changes have not yet had a full desktop-package run. The frontend receiving candidate is frontend/chat-cleanup at 30a56fba0dc2b44a2252d70188f8282df4bae359. Its review note was read; its rendered evidence uses an invented host and does not verify this incomplete host lifecycle.


## Lifecycle verification in progress

Focused lifecycle and observer checks passed before the later recovery and ordinary-view additions. The complete desktop package then failed after 600.207 seconds without naming a failing test; the same command passed the bus package in 116.227 seconds, broker in 1.179 seconds and host in 0.497 seconds. This failure remains unresolved and is not replaced by later passes. A debugger attachment to that isolated test process could not resolve Go symbols; it was detached with the process left running. No source logging or assertions were changed for that investigation.

The expanded focused run passed the desktop cleanup cases in 13.809 seconds and retained-recipient cases in 0.879 seconds. It includes every durable recovery phase, loaded-session disclosure, notification retention, rollback-copy redaction and observer consistency. A full desktop run with JSON test events and a five-minute bound is in progress. Its evidence is in the temporary file engine-cleanup-desktop-events.jsonl. The final tree still requires vet and the complete desktop gate.

The org-chart query contract is docs/design/org-chart-query.md at 77559d93. Front End agreed the DTO in message 8647, and Release agreed captured-generation handling in message 8650. Its App implementation follows the cleanup host step. No provider history query is part of that call.


## Named-chat fixture stall

The JSON run failed at 300.198 seconds with TestCreateNamedChatWithAModelTheProviderLacksRemovesTheChat still running. A separate thirty-second reproduction captured the timeout stack in %TEMP%/engine-cleanup-stall-7b263efc9fb9409591253ee0aa8837c7. It shows Provider.Wait inside ProviderLifecycle.Stop, called by releaseUnusedCleanupProviders during the failed named-chat rollback. The fixture installed a provider generation without its lifecycle watcher, so no owner could finish that generation after Close. Production starts that watcher with the provider.

The fixture now starts the real lifecycle watcher and joins it during cleanup. Its refusal and half-configured-chat assertions are unchanged. Release confirmed ownership of this fixture correction while retaining named_chat.go and the receipt-based creation changes. The bounded reproduction, the two full failures and their durations remain part of the record. Focused verification and the final full rerun remain pending.


The corrected named-chat fixture and focused cleanup suite passed in 5.829 seconds. The session-preferences fixture had the same missing watcher and now supplies one as well, preserving its existing saved-preference assertions. In the next full diagnostic run both fixtures passed (0.320 and 0.420 seconds respectively). That run continued to TestAFileReadForADocumentIsWatched near the end of the package and reached the investigator-imposed five-minute cap at 300.222 seconds. It did not stall again at either provider fixture. The next run uses the package's normal ten-minute budget with crash capture and ordinary package output. No test assertion deadline or expected result was relaxed. Vet passed for the desktop, bus, broker and host packages before these final fixture-only and bounded-report changes; a final-source repeat is required.


## Immutable host candidate

The full desktop package passed on source a285ebf1b7897251d342c251c1b467032aa2682e in 194.067 seconds through FIFO, with the chatcleanup_acceptance tag and the package's normal timeout. Crash capture is retained in %TEMP%/engine-cleanup-gate-d0886c023e2441698cd44e829618adea. The earlier red runs and fixture correction remain recorded above. A final-source shared-package run and vet are queued behind the priority document work. This is an integration candidate; Release's durable model and the drawing-lease combination remain separate gates.

The test-only binding is cmd/dinah-desktop/chat_cleanup_acceptance_test.go. Its install, sweep and observe functions have the contract's package-local shapes. Observe reads raw persisted settings rather than invoking settings migration. Retained content stays observable in the archive; finalization preserves no chain relationship beyond minimal member tombstones, so independent per-member post-finalization observations should use the fixture's seeded identities. The fixture helpers in chat_cleanup_lifecycle_test.go supply invented transcripts, settings and stable bus places. Provider hooks receive provider and unmodified native ID, and the real lifecycle fixture watcher is required when a test expects the final provider to stop. No product test route was added.

The frontend candidate remains 30a56fba. It has not yet been combined with this host tree. Independent Quality acceptance has not run. No merge, push, installation or real-home write has occurred.


The isolated combined branch is engine/chat-cleanup-combined in .worktrees/engine-chat-cleanup-combined. Merge 61dd1e66 combines host source a285ebf1 and Front End's immutable 30a56fba without conflict. This combination is ready for fixture binding, with the host full-package pass stated above; the combined asset/tree gate and rendered checks remain pending. It is not a product merge or installation. Release's model inventory seam and drawing-authority combination remain explicit integration gates. Engine retains host ownership and Front End retains page corrections.
