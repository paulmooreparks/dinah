---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:13Z
ordinal: 7
note: "Re-ran the census grep at merged commit 1648be0: grep -rln \"TakesWorkUp\\b\\|\\.States()\\|HoldsState\" --include=\"*.go\" . | grep -v _test.go | grep -v '.claude/worktrees' resolves to the same 8 production files the spec names. No ninth hit, including after the trunk merge which added internal/rename (no match there)."
---
No production call site of TakesWorkUp/States/HoldsState beyond internal/bench/bench.go changes behaviour. Test hook: grep -rln "TakesWorkUp\b\|\.States()\|HoldsState" --include="*.go" . | grep -v _test.go | grep -v '.claude/worktrees' resolves to the same 8 files the spec's call-site census names, both before and after implementation. Weaker than a behavioural proof (shows no new file matches the pattern, not that an existing file's use could not itself have silently changed meaning); ACs 1-6 cover that half.