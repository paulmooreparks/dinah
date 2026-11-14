# Decisions engine work

The assignment in the Desktop Team on 14 November covers durable person questions, partial answers, plain group attention and confirmed broadcasts. The accepted contract is docs/design/decisions.md. Engine owns branch engine/decisions from master 29a02bc1 in .worktrees/engine-decisions. Front End owns the separate Decisions window.

## Implementation

The bus stores a structured request together with its root message. The ask_person tool requires a recommendation for each question and explains that progress reports belong elsewhere. The window methods return open requests and plain group mentions, commit numbered answers as person replies, and reconcile durable operation receipts. Terminal refusals retain a code and a readable reason. Answer replies contain the question text and the expanded recommendation or free-text answer.

Broadcast previews freeze text and recipient identities for five minutes. Confirmation commits all addressed person messages with the receipt. The existing watcher and background scheduler admit closed chats. Removed or retired targets cause a terminal refusal before any delivery; confirmation never silently changes its target set. A removed asking chat or a missing group membership similarly refuses an answer. External terminal sessions remain visible in previews, but the window cannot open those terminals.

## Verification record

Focused Go tests passed before the final input-validation cleanup. They exercised persistence across database connections, partial answers, stale revisions, stored rejections, identical retries, operation conflicts, malformed input, concurrent answers, rollback after reply insertion, group membership, person replies and dismissal, cursor paging, frozen/expired/consumed previews, removed/retired targets, MCP tool guidance and host bridge classification. A fake-provider host test queued five closed chats through a two-place background limit. Another host test selected a working runtime chat without a bus hook turn. No visible window was created.

The first focused invocation had a fixture compilation error because BeginHookTurn was called with the wrong signature. The fixture was corrected; subsequent focused invocations passed. No rendered Decisions UI, external-terminal waking, full browser suite, live request/broadcast, push, merge or installation has been performed.

The complete touched-package gate ran under the shared FIFO on implementation 6b12a0d6. The command `go test ./internal/bus/... ./internal/mcpserver/... ./cmd/dinah-desktop/... -count=1` exited zero. The bus package passed in 51.635 seconds, MCP server in 9.861 seconds and desktop in 180.693 seconds. The desktop product package had no test files. The branch was clean throughout this run; subsequent changes to this record do not change executable source.

The focused race gate waited its FIFO turn and ran `go test -race ./internal/bus/... ./internal/mcpserver/... ./cmd/dinah-desktop/... -run 'Test(Decision|AskPerson)' -count=1` on the same executable source. It exited zero with bus 8.577 seconds, MCP server 1.592 seconds and desktop 3.022 seconds. The Windows desktop linker printed `.rsrc merge failure: multiple non-default manifests`; the test binary then ran and passed. No race was reported. This records the diagnostic rather than calling the link clean.

## Handoff

The implementation is ready for Lead review and Front End integration against the accepted methods and DTOs. The commits are a75815aa for the initial contract, a83674cb for terminal refusals and 6b12a0d6 for implementation, tests and the final contract details. Engine retains ownership until review accepts the branch. The rendered window and its saved drafts remain Front End's verification responsibility. Nothing on this branch has been pushed, merged or installed.

## Follow-up from Lead review

The 14 November review requested shared agent guidance and a way to clear unavailable requests. The chat communication text now directs person-only decisions through ask_person with recommendations, while person action recipients remain for results and notices. The MCP server's rules carry the same distinction for external agents without assuming their server has the window's name. The rest of both instruction texts remains unchanged.

The contract amendment at 33fe6f05 adds canDismiss and dismissalReason to Decision. Plain mentions remain dismissible. Structured requests become dismissible when the asking chat is removed or retired, or when their original group destination or membership is gone. Dismissal rechecks the current revision and availability, closes the durable row without fabricating answers, and retains an idempotent receipt. A restored destination refuses dismissal with a durable dismissal_unavailable receipt.

Focused checks passed for all four unavailable-destination cases, clearing the row after a partial answer, stale dismissal revisions, receipt reconciliation, retry without another reply and a destination restored between display and dismissal. The updated shared guidance checks also passed before the final external-server wording correction. Front End has the committed DTO amendment and owns displaying the legal action.

The complete follow-up gate ran under FIFO on ef616c57 with a clean branch. The command `go test ./internal/bus/... ./internal/communication/... ./internal/mcpserver/... ./cmd/dinah-desktop/... -count=1` exited zero. The bus package passed in 70.660 seconds, communication in 0.402 seconds, MCP server in 22.830 seconds and desktop in 168.358 seconds. The desktop product package had no test files. The earlier focused race result belongs to 6b12a0d6; it was not rerun for this follow-up. No rendered window, live bus mutation or desktop automation was used, and nothing was pushed, merged or installed.

The follow-up source is ef616c572a15187a9f93da968b063bcc338f4ce7 after contract amendment 33fe6f05. It is ready for combined review with Front End, with Engine retaining ownership for findings.
