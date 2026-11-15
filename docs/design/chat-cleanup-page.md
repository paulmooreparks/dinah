# Clean Up Chats

The Clean Up Chats page follows the canonical Architectural Principles in `C:\Users\paul\OneDrive\Documents\Architectural Principles.md`. The page opens from a visible button above the Chats list and from the Chat menu. Its workspace, agent program, age and closed-only filters are recorded in the workbench URL, so reopening an address restores the same view.

The host supplies every locally known chat, including closed chats in other workspaces, with its title, workspace, agent program, last activity, message count and eligibility. The page does not open chats to collect metadata or infer permission to delete them. The page is stacked on Engine's contract commit `87d96344` and consumes the amended contract in `docs/design/chat-cleanup.md`.

The list starts with the oldest chat. Selecting rows changes only their checkboxes and the count in a reserved toolbar box. The removal button keeps one width. Refusal text has a reserved box in each row, and long titles and reasons scroll inside their boxes. Narrow windows use the same fixed-height rows with stacked metadata. Filtering is an explicit change to which rows are shown.

After deletion, the row stays in its original position with its result and an unavailable checkbox until the person refreshes. This prevents another chat from moving under the pointer. The Recycle Bin has a separate scrolling region, with deletion and expiry dates and Restore and Delete Now buttons. Empty Recycle Bin applies final deletion to every retained chat. Restoring updates the result in place. No automatic refresh reorders an active list.

The staging confirmation states that the selected chats will move to the Recycle Bin, remain restorable for 30 days and leave the agent program's own history untouched until final deletion. All single-chat paths use Delete Chat and the same staging operation. The page never calls DeleteSession or HideSession. Engine owns the retained files, dormant runtime state, refusal of new messages, restoration and expiry. The page uses Engine's batch results and reports each failure beside its chat while allowing the remaining deletions to finish.

Paul's final-deletion decision is applied only by the host. The bin shows each row's authoritative final-deletion explanation. Delete Now and Empty Recycle Bin display the corresponding host confirmation verbatim. If the agent program cannot delete its own copy, or fails to do so, the local deletion still completes and its result explains the remaining copy. The page accepts no policy override.

Cleanup changes only the chat's own records and runtime state. It does not change documents, document drafts, comments, suggestions, attachments, publication records, workspace files, remote services, credentials or tool registrations. A reference from a chat gives cleanup no ownership of the referenced record. The host revokes all chat leases when it enters the bin. Restore cannot reuse an old lease.

The frontend preserves the person's unsent chat message while the chat is in the bin. Final deletion forgets that chat composer value; this is separate from document drafts. The host also erases these values durably and rejects stale writes for permanently deleted identities. Deletion events dispose of cached controllers and list rows without querying an agent program. Restore applies its authoritative row to the closed-chat metadata cache without provider discovery.

The frame suite will measure row positions, the toolbar and surrounding controls before selection, during delayed calls and after mixed outcomes. It will also check the narrow layout, URL restoration, refused rows, confirmation cancellation and restoration with invented chats. Verification uses an isolated headless page and does not remove real chats.
