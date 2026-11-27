# Team start model reporting

The repair is on `engine/team-model-report`, based on `7aa7797d`.

The first regression failed because requesting an unavailable model returned an empty `notApplied` list while the member retained its default. Both start actions now return the effective model ID and a refusal entry identifying the requested model when it cannot be applied. The group start line uses the effective model.

The member continues running after a model refusal, matching the existing postcreation setting-refusal contract. Accepted settings remain applied. Both action output schemas describe the additive fields. The decision is recorded in `docs/design/team-duplication.md`.

Focused tests pass for an unavailable explicit model with another accepted setting, accepted and unavailable inherited models, provider refusal, a setter that retains the default, and a provider without a model setting. Both action versions and their output schemas are covered. The original regression failed before the implementation and passed afterwards.

The first uncached desktop package gate found only the old test requiring a four-field start@1 response. That test now requires the six-field response, including the effective inherited model and an empty refusal list, while retaining its input, permission and remit checks. The uncached package rerun passed: `pwsh scripts/go-gate.ps1 ./cmd/dinah-desktop -count=1` (352.661 seconds). No browser suite is needed because the change touches no page code. The focused staged-file guard and full uncached profile guard passed without skips. The full guard used `DINAH_FORBIDDEN_TERMS` with `scripts/go-gate.ps1 -GoTestArgs @("./internal/profile/", "-count=1", "-v")` and completed in 10.835 seconds. The commit and patch scan result accompanies the handoff. No merge, push or install has been performed.
