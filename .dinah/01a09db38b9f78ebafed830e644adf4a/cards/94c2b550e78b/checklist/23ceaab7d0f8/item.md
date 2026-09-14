---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:31Z
ordinal: 4
note: "Verified at 5a88bec. Planted \"Dinah has no restore.\" as its own paragraph at the end of internal/guide/guides/verbs.md, a guide the retired check never read. `go test ./cmd/dinah/ -run TestNoGuideDeniesACommandTheToolHas -v` compiled, ran, and went red with: internal/guide/guides/verbs.md says \"has no restore\" and `dinah restore` is a command this build carries: dinah has no restore / (a command name standing as an ordinary noun trips this check; reword the sentence rather than exempting it). The failure names the file path as required. Restored from a byte-identical copy (cmp clean) and the check went green again."
---
The sentence "Dinah has no restore." planted as its own paragraph in internal/guide/guides/verbs.md fails TestNoGuideDeniesACommandTheToolHas, and the failure names internal/guide/guides/verbs.md. The plant is restored from a byte-identical copy afterwards and the check goes green again.