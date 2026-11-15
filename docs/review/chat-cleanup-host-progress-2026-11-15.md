# Chat cleanup implementation record

The host work is on `engine/chat-cleanup` in `.worktrees/engine-chat-cleanup`. The contract is `docs/design/chat-cleanup.md`. This branch is incomplete and is not an implementation handover.

## Completed foundations

The bus can retain a recipient's queue while refusing new messages, delivery and wakeup. Restore reactivates the address with its old deliveries dormant until explicit registration or accepted new input. Ordinary group and Decisions views exclude binned recipients. Final queue disposal remains distinct from retained removal.

The cleanup DTO and local listing read saved messages without starting a provider. The temporary chain helper includes bindings, current attempts and archived attempts. Release and Engine agreed the membership boundary in bus messages 8552, 8572 and 8575. The interim implementation is logical_chat_members.go. Listing consumes that resolver, which excludes distinct recipients and rejects cyclic or ambiguous committed ownership. Unknown allocation outcomes carry no invented native identity. The durable model design record remains Release's responsibility. Transcript ancestry alone does not confer ownership. Cleanup will consume Release's single logical-membership function after its concrete types are agreed.

The archive helper moves chat-owned files outside startup scan folders and resumes interrupted moves. Restore refuses conflicting active files without overwriting either copy. Its settings payload has a narrow type which cannot serialize workspace records, templates or credentials. UI-state saves preserve retained composer drafts and filter permanent tombstones under the same mutex that finalization must hold.

## Verification

The full bus package passed through FIFO after the retained-recipient changes. Focused desktop tests passed for chain membership, transcript-only fork exclusion, local closed-chat counts, archive moves, interrupted resume, conflicting restores, settings ownership and stale composer saves. The existing UI-state tests passed with the stale-save filter. The first broad run found the missing ListChatCleanup entry in pageMethods. That entry was added as a query. The updated tree then passed the complete desktop package in 179.287 seconds and sessionhost in 20.748 seconds through FIFO. These passes verify the foundation only; the next lifecycle handover requires a full desktop run from its final tree.

## Remaining work

The lifecycle still needs staged removal, whole-chain eligibility and generation checks, runtime disposal, provider unload/deletion handling, restore, expiry and interrupted-operation reconciliation. The compatibility methods and page method classification still need wiring. Final deletion must purge all owned records, including composer drafts, while leaving referenced document records untouched. The runtime must revoke drawing and applet authority and cancel pending authorized work, with fresh authority only on a subsequent open. The test-only acceptance bindings and combined frontend/Quality execution remain required.

Release owns the durable logical-chat model and its resolver. Engine owns cleanup lifecycle, archive and host DTO files. Front End owns presentation files and fixtures. No push, merge, installation, remote write or real-home operation has been performed for this cleanup work.


The interim membership and local listing passed focused cleanup tests after the full foundation run. Those later resolver changes have not yet had a full desktop-package run. The frontend receiving candidate is frontend/chat-cleanup at 30a56fba0dc2b44a2252d70188f8282df4bae359. Its review note was read; its rendered evidence uses an invented host and does not verify this incomplete host lifecycle.
