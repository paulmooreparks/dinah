---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:42Z
ordinal: 7
note: "Verified 2026-09-12 on head 995636e: go test ./internal/bench -run TestMigrateNumbersIsDeterministicAndIdempotent passed within the 41-test -run match. Two independent copies of the same old-format fixture migrated separately produced byte-identical card-numbers.txt files, and a second migration over the already migrated workbench wrote no file, reported no finding, and left every anchor's bytes unchanged."
---
The migration is deterministic and idempotent. `go test ./internal/bench -run TestMigrateNumbersIsDeterministicAndIdempotent` passes: two independent copies of the same old-format fixture migrated separately produce byte-identical `card-numbers.txt` files, and a second migration run over an already migrated workbench writes no file, reports no finding, and leaves every anchor's bytes unchanged.