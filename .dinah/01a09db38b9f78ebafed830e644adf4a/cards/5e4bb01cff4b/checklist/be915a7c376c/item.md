---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:00Z
ordinal: 6
note: "Test-stage re-verification, both directions armed independently on the merged tree. (1) Shifted internal/mcp/mcp.go:166 down one line: guard reported both the new undeclared call site (mcp.go:167) and the stale declared entry (mcp.go:166) in the same run. Restored via git checkout, green. (2) Appended `var armingProbe = msg.For(msg.Base)` to internal/verb/read.go: guard failed naming read.go:912 as undeclared. Restored via git checkout, green. Declared inventory (mcp.go:166, tools.go:165, tools.go:201) confirmed to match `git grep -n 'msg\\.For(' -- '*.go' | grep -v _test.go` exactly on the merged tree, reflecting dinah-282's line move."
---
internal/verb/reachability_test.go declares PinnedCallSite, declaredPinnedCallSites (seeded with exactly internal/mcp/mcp.go:166, internal/mcp/tools.go:102 and internal/mcp/tools.go:138), and TestEveryLanguagePinnedCallSiteIsDeclared, which parses cmd/dinah, internal/mcp and internal/verb with go/parser, fails on any msg.For call site pinned to a literal or to msg.Base that isn't declared, and fails on any declared entry the parser no longer finds.