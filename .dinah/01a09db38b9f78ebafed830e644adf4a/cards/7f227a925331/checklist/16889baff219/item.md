---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:18:42Z
ordinal: 3
note: "Verified 2026-09-12 on head 995636e: go test ./cmd/dinah -run TestFilingACardAppendsOneRegistryLine passed in 0.13s. The test asserts exactly what the criterion names: after dinah add, card-numbers.txt gains one line matching ^[1-9][0-9]* [0-9a-f]{12}$ whose identifier is the new card's directory, and the anchor carries no number key."
---
Filing a card writes one registry line and no frontmatter number. `go test ./cmd/dinah -run TestFilingACardAppendsOneRegistryLine` passes: after `dinah add`, `card-numbers.txt` has gained exactly one line matching `^[1-9][0-9]* [0-9a-f]{12}$` whose identifier is the new card's directory name, and the card's anchor carries no `number:` key.