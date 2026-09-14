---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:32Z
ordinal: 15
note: "Verified at 5a88bec by reading the diff. `git status --short` and `git diff --cached --stat` name eight files, and docs/quick-start.md is not among them: cmd/dinah/guide_guard_test.go, cmd/dinah/guide_pin_test.go, cmd/dinah/references_guide_test.go, cmd/dinah/restore_test.go, internal/bench/resolve_archived_half_test.go, internal/guide/guidepin/guidepin.go, internal/guide/guidepin/guidepin_test.go and internal/guide/guides/references.md. The widened check's corpus is guide.Topics() alone, which does not include the quick start. Grepping the added lines for exempt, allowlist, allow-list, suppress, skiplist and ignorelist returns exactly one hit, and it is the failure message telling a reader to reword a sentence RATHER THAN exempting it. No exemption list, allowlist or suppression mechanism is added anywhere in the diff."
---
docs/quick-start.md is unchanged by this card and is not in the widened check's corpus, and no exemption list, allowlist or suppression mechanism is added anywhere in the diff. Verified by reading the diff.