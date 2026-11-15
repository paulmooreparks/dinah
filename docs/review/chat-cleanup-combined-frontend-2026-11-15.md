# Combined cleanup frontend verification

Front End verified the immutable combined source `61dd1e66e77f89944156b320053714a44063ef95` in its own `frontend/chat-cleanup-combined-check` worktree. That merge combines Engine's host implementation `a285ebf1b7897251d342c251c1b467032aa2682e` with Front End's UI `30a56fba0dc2b44a2252d70188f8282df4bae359`. No source changes were needed.

The five touched browser suites passed together under FIFO ticket 2179 in 49 seconds. They were `app-ui3`, `chat-cleanup`, `drafts`, `menus-through-updates` and `recovery-card`. The cleanup suite checks filters, mixed results, refusal rows, switched-program information, Restore, final deletion, confirmation text, ordinary-cache disposal and restoration without discovery, composer drafts, narrow layout and frame stability. The committed screenshots and frame measurements are in `docs/review/chat-cleanup-combined-frontend-2026-11-15/`.

Frontend ownership, UTF-8 and release-asset checks compiled and passed with `chatcleanup_acceptance` under FIFO ticket 2178 in 0.421 seconds. Front End also compared the implemented JSON fields, method signatures and removal/restoration event payloads with the page's consumers. They match the frozen contract. The page uses authoritative result rows and final confirmations and does not refresh provider history after removal or Restore.

The browser command was `pwsh ./cmd/dinah-desktop/frontend/e2e/run.ps1 -Only app-ui3,chat-cleanup,drafts,menus-through-updates,recovery-card -Jobs 1 -Out $env:TEMP/chat-cleanup-combined-frontend-20261115`. The asset command was `pwsh ./scripts/with-test-lock.ps1 go test -tags chatcleanup_acceptance ./cmd/dinah-desktop -run 'Test(FrontendJobsStayInTheirHomes|FrontendRulesFindTheirHomes|ReleaseAssetsExcludeLocalFixtures|ThePagesTextFilesAreUTF8)$' -count=1`.

These are rendered-page checks with invented bridge responses plus compiled asset checks on the combined source. They do not prove the native window, durable lifecycle, logical-chat adapter, startup or lease revocation. Engine and Quality retain those gates and the joint acceptance report. No native program was installed, no real home or remote service was used, and no visible window was opened.

Front End found no blocking presentation or DTO discrepancy in this candidate. Engine can include this record in the joint handover after the remaining gates pass. Front End retains any UI corrections found by that work.
