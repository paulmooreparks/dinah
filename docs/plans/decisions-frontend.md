# Decisions window verification

The implementation is committed at `5754bf55b0d2f360f65dac0207424597ed81e0bf` on `frontend/decisions`, based on `29a02bc1`. The service methods and DTOs were checked against Engine's `ef616c572a15187a9f93da968b063bcc338f4ce7`, including the terminal rejection protocol and explicit unavailable-request dismissal. This branch contains no backend changes and has not been merged, pushed or installed.

## Delivered behavior

View > Decisions and the Decisions launcher open the request list. Numbered questions include their recommendation and suggested choices. As Recommended prepares that question's answer for Send; it does not submit an answer by itself. The displayed destination and root message come from the selected request. Plain group messages have a free reply field. The engine's canDismiss affordance exposes plain-message and unavailable structured-request dismissal, with confirmation and the engine's explanation.

Each request has a saved draft and reviewed-content fingerprint in the existing UI state store. A changed question or destination requires another review. New arrivals append fixed-height list rows without moving the selected detail, replacing its text or taking focus. The provisional master-detail layout has its own stylesheet. It opens maximized, and a narrow window shows either the list or the selected detail with a Requests return button.

The page persists an operation identity and immutable payload before submission. Lost or invalid receipts remain unconfirmed. Reconciliation queries the engine, repeats only the identical call when its result is unknown, and accepts committed or rejected terminal receipts. Rejection preserves the person's draft and displays the reason. A corrected attempt uses a new identity. A saved request snapshot allows reconciliation after a fully answered row has disappeared from the open list. Invalid saved operations fail closed without being executed or replaced.

Broadcast to Active and Broadcast to All are commands with separate addressable compositions. Each obtains and displays the complete frozen preview count and title list before asking for confirmation. Cancelling sends nothing. Text edits invalidate the preview. A terminal refusal permits a fresh preview, and a confirmed preview cannot be submitted again through the same composition without previewing again. The page opens no recipient windows and owns no background scheduling policy.

## Verification

The ten new headless suites are `decisions-list`, `decisions-answers`, `decisions-drafts`, `decisions-delivery`, `decisions-broadcast`, `decisions-errors`, `decisions-races`, `decisions-narrow`, `decisions-pages` and `decisions-saved-state`. All ten passed in 38 seconds at `%TEMP%/decisions-complete-20261114`. Their failure record is empty.

The suites cover structured recommendations and choices, plain replies, partial revisions, destination disclosure, draft reload and storage failure, changed content review, committed and rejected receipts, exact retries, reload after a closed-row lost receipt, broadcasts with cancellation and fresh previews, unavailable-request dismissal, malformed reads and receipts, navigation during the pre-send save, earlier-page refresh, stable geometry/focus and corrupt saved operations. The narrow and desktop screenshots were inspected.

The preceding development run passed nine of ten suites and failed the closed-row recovery driver because it attempted to click the closed row after reload. That driver now tests recovery through the saved selection and receipt, without trying to find an open row that no longer exists. The failure remains in `%TEMP%/decisions-recovery-20261114/failures.txt`. Earlier eight-suite and six-suite checks also passed.

The final run of these ten suites plus the existing `app-ui3` and `notifications-popover` checks passed all twelve suites in 60 seconds under FIFO, with exit zero. Its record is `%TEMP%/decisions-final-20261114`; `failures.txt` is empty. The implementation, fixture and guard files remained identical to `5754bf55` throughout that final run. ESLint passed for both new production scripts, and diff whitespace checks passed. The ten new suites are registered in `scripts/guards.json`.

## Limits and handoff

The browser bridge uses synthetic service responses and isolated browser state. It does not prove live bus persistence, recipient waking, native shell restart or external-terminal delivery. Engine owns those backend checks. No Go suite was rerun on this frontend-only branch, and no full browser suite ran. The existing 32 MiB UI-state cap remains in force; storage errors preserve the current draft, show the failure and prevent a new send.

The shared mockup remains unapproved, so this work does not claim final visual acceptance. Dinah Team Lead should review this branch with Engine's matching contract and source, then integrate both together. No service-support question remains open for this window.
