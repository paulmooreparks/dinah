---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 4
note: "Armed with one throwaway grouped entry added to the commands table in cmd/dinah/commands.go. Red: \"docs/quick-start.md:105 says fifty-one commands, and the derivation groupedCommands yields 52, which the documents spell fifty-two\". This is dinah-446's own arming re-run against the replacement, so the removal of TestTheQuickStartCountsTheCommandsTheBinaryOffers is paid for. Entry removed, green."
---
A prose figure that disagrees with the binary is caught from the binary side. Arming: add one throwaway grouped entry to the `commands` table in `cmd/dinah/commands.go`, run `go test ./cmd/dinah -run TestEveryDerivedProseFigureMatchesTheBinary`, and watch it fail naming `docs/quick-start.md:105`, the figure `fifty-one`, the derivation `groupedCommands`, and the value fifty-two. Remove the entry and watch the run go green.