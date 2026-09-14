---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:35Z
ordinal: 8
note: "test: cmd/dinah/exithold_test.go#TestAnOperatorOwnedItemIsReopenedByAnybody, which files an open_question with --owner operator, resolves it as the operator, asserts it really is resolved on disk, then reopens it as \"sam\" asserting exit code 0 and the item back at pending. The assertion that it really was closed first is what stops the case passing on a build where reopen refused a pending item for an unrelated reason. observed before: fail, after: pass. Not separately armed: the arming proof on AC-7 removes the closeItem check, which is the code this criterion asserts does NOT reach Reopen, and Reopen does not route through closeItem at all (internal/verb/checklist.go:190 builds its own withItem closure), so the guard that would catch a mistake here is the AC-7 arming plus this run."
---
**End-to-end, run against the built binary, proving Reopen is deliberately unrestricted:** an `owner: operator` item closed by the operator in AC-7 is then reopened by a *different*, non-operator actor via `dinah reopen`, and this succeeds (exit code 0, item state returns to `pending`).