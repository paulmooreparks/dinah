---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:00Z
ordinal: 10
note: "Test-stage re-verification via independent Python sweep of en.json for the bare word \"owner\": exactly 18 keys, matching the criterion's list key for key. `go test -run TestATranslationUsesTheDeclaredWord ./internal/msg/` passes on the merged tree (already includes the 6 German + 2 Hindi backfills)."
---
The glossary's "owner" term triggers on exactly the 18 keys where the English word "owner" appears (check.attach.2, check.block.2, check.card.5, check.claim.2, check.claim.3, check.comment.2, check.join.2, check.leave.2, check.pull.1, check.rename.3, check.whoami.1, check.workbench-field.3, check.workbench-field.4, check.workstream-field.4, column.states.owner, flag.actor.summary, refusal.dinah.takes-no-work, check.claim-where-no-work-is-taken), all referring to the same actor-ownership concept with no second sense to carve out; TestATranslationUsesTheDeclaredWord passes against the corrected corpus (six German entries backfilled to contain "Akteur", two of those six also backfilled in Hindi to contain "स्वामी").