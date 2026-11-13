# Decisions engine work

The assignment in the Desktop Team on 14 November covers durable person questions, partial answers, plain group attention and confirmed broadcasts. The accepted contract is docs/design/decisions.md. Engine owns branch engine/decisions from master 29a02bc1 in .worktrees/engine-decisions. Front End owns the separate Decisions window.

## Implementation

The bus stores a structured request together with its root message. The ask_person tool requires a recommendation for each question and explains that progress reports belong elsewhere. The window methods return open requests and plain group mentions, commit numbered answers as person replies, and reconcile durable operation receipts. Terminal refusals retain a code and a readable reason. Answer replies contain the question text and the expanded recommendation or free-text answer.

Broadcast previews freeze text and recipient identities for five minutes. Confirmation commits all addressed person messages with the receipt. The existing watcher and background scheduler admit closed chats. Removed or retired targets cause a terminal refusal before any delivery; confirmation never silently changes its target set. A removed asking chat or a missing group membership similarly refuses an answer. External terminal sessions remain visible in previews, but the window cannot open those terminals.

## Verification record

Focused Go tests passed before the final input-validation cleanup. They exercised persistence across database connections, partial answers, stale revisions, stored rejections, identical retries, operation conflicts, malformed input, concurrent answers, rollback after reply insertion, group membership, person replies and dismissal, cursor paging, frozen/expired/consumed previews, removed/retired targets, MCP tool guidance and host bridge classification. A fake-provider host test queued five closed chats through a two-place background limit. Another host test selected a working runtime chat without a bus hook turn. No visible window was created.

The first focused invocation had a fixture compilation error because BeginHookTurn was called with the wrong signature. The fixture was corrected; subsequent focused invocations passed. The final touched-package and focused race results will be appended after completion. No rendered Decisions UI, external-terminal waking, full browser suite, live request/broadcast, push, merge or installation has been performed.
