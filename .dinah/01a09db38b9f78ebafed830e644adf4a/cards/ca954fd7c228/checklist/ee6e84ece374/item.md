---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:17:24Z
ordinal: 13
note: "citation: scheme=command, target=npm run test:unit under editors/vscode and go test ./... from the repository root, on the branch with current trunk merged, observed both passing. origin/main was still 65a80ad at merge time, so the merge was already up to date. The unit run reports \"Ran 554 unit test(s) from 33 file(s), 554 passing\"; the same command with the two new compiled files set aside reports 540 from 31, so the run registered all 14 new tests rather than exiting zero on a file that registered none. go test ./... passed every package including internal/profile, with no timeout. The VS Code integration suite was not run. gofmt -l . is empty, go build ./... and go vet ./internal/msg/... are clean, and npm run lint and npm run check-types both pass."
---
Both suites pass on the branch with current trunk merged into it: `npm run test:unit` under `editors/vscode/` and `go test ./...` from the repository root, the latter including `internal/profile` whatever the diff names. The unit run's own summary line reports more tests than the same command reports on trunk, since a run that registered none of the new file's tests exits zero and prints a passing summary. The VS Code integration suite is not run.