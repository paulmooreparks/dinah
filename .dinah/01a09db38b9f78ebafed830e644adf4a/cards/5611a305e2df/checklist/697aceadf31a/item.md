---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:17:40Z
ordinal: 3
note: "Armed. Red: \"docs/quick-start.md:105 says fifty-two commands, and the derivation groupedCommands yields 51, which the documents spell fifty-one\". Names the document, line 105, the figure as written, the derivation, and the binary's value. Restored, green."
---
A prose figure that disagrees with the binary is caught from the document side. Arming: change `fifty-one` to `fifty-two` at `docs/quick-start.md:105` and change that entry's `figure=` to match, run `go test ./cmd/dinah -run TestEveryDerivedProseFigureMatchesTheBinary`, and watch it fail naming the document, line 105, the figure `fifty-two`, the derivation `groupedCommands`, and the value the binary yields. Restore both and watch the run go green.