# Workspace selection during a pending switch

When a person chooses an existing workspace through the selector or a recent-workspace entry, the last choice wins. If B is opening and the person chooses A, Dinah finishes B's host call and then selects A. The selector keeps showing A while that work is pending.

The page allows one selection operation in flight. It retains one latest requested destination, replacing an intermediate queued choice when another arrives. A successful response is applied to the page only when it matches the current request. Selecting the in-flight destination again uses that response rather than submitting a duplicate call.

Saved workspace choices continue to use SelectWorkspace with the opaque record ID. Legacy folder choices continue to use SetWorkspace. No host method, authority check or persistence behavior changes. The missing-folder confirmation and folder-creation checks remain part of the serialized selection operation.

If the last choice fails or its missing-folder confirmation is declined, the selector returns to the workspace that actually opened. The page retains the last successful response while a queued choice is pending so a failure after a superseded success can show the correct workspace. Failures remain visible through the existing error reporting. A later choice can still proceed.

Selection and completion invalidate older catalog refreshes. Label redraws preserve the pending destination, including when workspace metadata is refreshed before the selection finishes.

The workspace-last-choice headless suite controls the host responses explicitly. It covers B then A, replacement of queued choices, repeated in-flight choices, changes during a queued call, failures and the recent-workspace route. Both saved-ID and legacy-path flows must finish at the last requested workspace with no overlapping selection calls. Frame samples must never paint the superseded destination in the selector.
