---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:10Z
ordinal: 2
note: "Test: TestEnumerateFindsBothWorkbenchesInAnAmbiguousRoot, internal/bench/rootenumeration_test.go. Rerun: `go test ./internal/bench/ -run TestEnumerateFindsBothWorkbenchesInAnAmbiguousRoot -v`. Green after the fix. Armed with the named mutation: replacing the ambiguity loop in enumerate with `if len(ambiguous) > 0 { seen[ambiguous[0]] = true; collected = append(collected, describe(ambiguous[0])) }` turns it red at rootenumeration_test.go:98 (\"answered 1 candidates, wanted both of its container's two\"), while AC-1's test stays PASS under that same mutation, which is why this is a separate case. Also red with the whole root probe removed."
---
A unit test in internal/bench builds a directory whose own .dinah holds two recognized workbenches and asserts enumerate(root) returns exactly two candidates, matching the two anchor directories compared as a set. Must go red if the ambiguity loop is changed to take only ambiguous[0] or is skipped entirely.