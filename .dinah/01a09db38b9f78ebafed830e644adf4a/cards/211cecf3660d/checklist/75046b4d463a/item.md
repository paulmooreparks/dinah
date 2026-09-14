---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:27Z
ordinal: 9
note: "Re-verified live: sha1 of every file under the scratch workbench's .dinah directory was identical before and after `dinah check` ran (diff of two full manifests was empty). Also go test ./internal/bench/ -run TestCheckReportsEveryItemColumnThatCannotHoldACard -v PASS, which asserts read-only per the workbench's own convention."
---
The new sweep in internal/bench/check.go performs no write to disk: a test asserting the fixture directory's contents are byte-identical before and after bench.Check() runs passes, consistent with every other sweep in that file.