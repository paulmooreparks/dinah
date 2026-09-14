---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:32Z
ordinal: 17
note: Verified at 5a88bec. The assertion is `if len(topics) == 0 { t.Fatal("the library names no guide topic, so the corpus this check sweeps read nothing") }`, sitting above the loop so an empty list cannot satisfy AC-1's count check with 0 == 0. Armed by shadowing `topics := guide.Topics()` with `[]string{}`; the run went red on exactly that sentence rather than passing. Restored byte-identically and green.
---
TestNoGuideDeniesACommandTheToolHas fails when guide.Topics() is empty, so a sweep that read no guide at all cannot pass by having scanned nothing. The failure says the topic list read nothing. Armed by shadowing guide.Topics() with an empty slice at the call site and confirming the check goes red rather than green.