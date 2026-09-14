---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 7
note: "Verified at 5a88bec by reading the message a planted failure actually printed, not by asserting its presence. The AC-4 plant produced: internal/guide/guides/verbs.md says \"has no restore\" and `dinah restore` is a command this build carries: dinah has no restore / (a command name standing as an ordinary noun trips this check; reword the sentence rather than exempting it). That carries the guide's file path (rather than its topic, which is the reviewer's major correction applied), the matched fragment \"has no restore\", the whole sentence, and the false-positive shape with its remedy. The first verb renders the path from the topic, since //go:embed guides/*.md at internal/guide/guide.go:17 is one file per topic."
---
The check's failure message names the guide's file path, the matched fragment, the whole sentence, and the known false-positive shape with its remedy, so a reader meeting it can tell a true finding from a command name used as an ordinary noun without opening the test. Verified by reading the message a planted failure actually prints.