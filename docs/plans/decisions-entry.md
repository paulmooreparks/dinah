# Decisions entry points

The entry-point work is on `frontend/decisions-entry`, based on `fb650e77`. The window's layout and mutation contracts remain unchanged. The frontend design record is `docs/design/decisions-frontend.md`.

The status bar shows the complete open-request count with a reserved-width badge. The page uses the new query-only `DecisionEntries()` App method, listed in pageMethods, to obtain `{count,items:[{id,messageId}]}` without downloading question bodies. The bus implementation shares the existing open-row predicate and identity construction with Decisions. Reads are coalesced, and bus changes invalidate an in-flight snapshot. An unavailable or malformed count shows `?` rather than zero.

New direct and group ask_person requests already produce durable attention cards through the existing watcher. This change reuses those cards and their per-chat grouping. It resolves a card's source message to an opaque Decisions identity, opens that row through as many exclusive-cursor pages as needed, and brings the row into view. Lookup, row-read and form-display failures preserve the card. Successful navigation dismisses only the notification identities captured before opening; later arrivals survive. Opening does not answer or dismiss the decision. Messages outside the open Decisions index keep their previous source navigation.

## Verification

The focused Go commands ran under the shared FIFO and exited zero:

- `go test ./internal/bus ./cmd/dinah-desktop -run 'TestDecision|TestAskPerson|TestEveryPageMethodIsListed|TestExitGuardReadsClassesFromPageMethods|TestFrontendJobsStayInTheirHomes|TestFrontendRulesFindTheirHomes|TestAMentionMakesOneCardAndTheHostIsTold|TestReadingAMentionKeepsItsCardUntilAnAnswerOrDismissal' -count=1`
- `go test ./internal/bus ./cmd/dinah-desktop -run 'TestDecisionEntries|TestDirectDecisionMakesRetainedAttention|TestEveryPageMethodIsListed|TestExitGuardReadsClassesFromPageMethods' -count=1`

The new Go cases cover a complete 103-request index beyond the first full page, partial and full answers, an empty non-null list, ordinary direct messages excluded from Decisions, plain group mentions and their dismissal. Watcher cases exercise direct and group structured requests with one durable card and one fake native notice despite repeated watcher passes. Dismissing attention leaves a durable request open.

Eleven affected headless browser suites passed. The ten-suite run covered `attention-kept`, `decisions-attention-entry`, `decisions-delivery`, `decisions-errors`, `decisions-list`, `decisions-pages`, `decisions-races`, `decisions-saved-state`, `decisions-status` and `mention-notification`. After tightening the form-display check, `decisions-attention-entry`, `decisions-status` and `decisions-narrow` passed on the final source. The launcher explicitly uses headless Edge and page-only events.

The new browser cases verify unchanged badge and neighbour rectangles for 0, 1, 99, 100 and 103 requests, full accessible counts, missing API behavior, unknown and malformed counts, and invalidation of a delayed snapshot. Card cases exercise an opaque deep link across four pages, structured and plain requests, source navigation fallback, failures in index/list/form reads, and a newer request arriving while an older card opens.

Artifacts are retained under `%TEMP%/decisions-entry-focused-20261114` and `%TEMP%/decisions-entry-final-20261114`; both failures.txt files are empty. Earlier development results remain under `%TEMP%/decisions-entry-first-20261114` and `%TEMP%/decisions-entry-second-20261114`. The first failed before startup because the new fixture referenced its API before initialization. The second passed the status suite and earlier card checks but timed out on a plain row supplied out of descending message order. Both fixture errors were corrected without changing production acceptance rules.

ESLint passed for the three changed production scripts. The repository's ESLint configuration excludes browser suites and reported those files as ignored when included, so no lint result is claimed for them. New tests are registered in scripts/guards.json without adding an exception.

The browser uses synthetic service responses. The Go watcher uses temporary bus/settings data and a fake notifier. No installed window, real native alert, real recipient or full browser/Go run was exercised. No push, merge, installation or shared-desktop input occurred. The branch is ready for Lead review, with ownership retained for findings.
