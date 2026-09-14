---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 1
note: Verified on branch dinah-478-every-guide-is-scanned-for-a-denial at 5a88bec. The assertion is `if scanned != len(guide.Topics())` in TestNoGuideDeniesACommandTheToolHas, cmd/dinah/guide_guard_test.go. Armed by replacing `topics := guide.Topics()` with `guide.Topics()[:7]` at the call site; `go test ./cmd/dinah/ -run TestNoGuideDeniesACommandTheToolHas` went red with "this check scanned 7 guides and the library names 8, so it read less than the corpus it claims", which is the count assertion rather than a sentence. Restored byte-identically (cmp clean) and green. The passing run logs "8 guides scanned, 526 sentences, against a roster of 56 commands".
---
TestNoGuideDeniesACommandTheToolHas reads every guide guide.Topics() names, and fails rather than passing when it reads fewer: the run counts the guides it scanned and fails unless that count equals len(guide.Topics()), which is 8 today.