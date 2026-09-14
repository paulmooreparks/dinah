---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:00Z
ordinal: 1
note: Test-stage re-verification on merged tree (branch + trunk, no drift; HEAD 5f7c921, trunk already ancestor). `go test -run TestCatalogEntryReadsWithoutFallback -v ./internal/msg/` PASS on the merged worktree.
---
internal/msg/msg.go exports CatalogEntry(tag, key string) (Entry, bool), returning the entry exactly as tag's own catalog carries it with no fallback to Base and no placeholder substitution; internal/msg/msg_test.go's new TestCatalogEntryReadsWithoutFallback asserts it against "de"/"word.yes" (found, non-empty Text), "qq"/"word.yes" (not ok), and "de"/"no.such.key" (not ok).