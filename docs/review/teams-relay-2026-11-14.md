# Teams relay failure review, 14 November 2026

The work is on `quality/teams-relay`, based on master `824054b3`. The branch worktree is `.worktrees/quality-teams-relay-fix`; `.worktrees/quality-teams-relay` is a separate detached bisect worktree.

## Cause and correction

This is a product defect. The retained attention renderer replaces the notification's supplied details with its request-count summary. Relay failures put their reason in those details, so the reason disappears from the full card. Both Open Group and Open Lead are still present. The original Teams assertion correctly requires all three and is unchanged.

The correction appends the latest request's details to the count summary. It follows the existing card policy of showing the latest request from each source. It does not change grouping, dismissal, routing or navigation. Missing details still produce only the summary.

The added `teams-relay-details` suite checks two relay failures from the same chat. The latest reason and remedy must appear beside the two-request count, the older reason must no longer describe the latest request, and both navigation actions must remain. It checks arrival, reload and redraw with invented persistent notifications.

## Regression boundary

A first-parent Git bisect runs only the Teams suite over today's integration history. Yesterday's `4fe5a52c` passes, and base `824054b3` fails the reported relay assertion. The original assertion exists at both endpoints.

The bisect uses `git bisect start --first-parent 824054b3 4fe5a52c`. Its runner returns bad only for the named relay assertion, good only for a clean suite, and skip for any other failure. No commit has required a skip.

| Revision | Teams result |
| --- | --- |
| 824054b3 | FAIL, the reported relay assertion |
| 4fe5a52c | PASS |
| 3e4a52f9 | FAIL, the reported relay assertion |
| 4248443a | FAIL, the reported relay assertion |
| 95c4aaa0 | PASS |
| e0e4a865 | PASS |
| 3d18f5b3 | FAIL, the reported relay assertion |
| 8f5add6e | PASS |

The immediate predecessor `8f5add6e` passed in 7 seconds. Git bisect identified `3d18f5b3c786746c10d92f90be7766ef70db738a` as the first bad integration commit, at 07:22:51 +08:00. Its source change is `820c69eba3a62813dd3fe75b8e114464d2e72182`. Source comparison shows that this change dropped supplied details. The completed bisect log is retained at `%TEMP%/quality-teams-relay-bisect.log`; the detached worktree has been reset to the original base.

## Verification of the correction

The source and regression test are committed as `a3ea8738`. `teams`, `teams-relay-details` and `attention-kept` passed together in 9 seconds, with an empty `failures.txt`. The original `teams.js` is byte-for-byte unchanged from the branch base. JavaScript syntax and Git whitespace checks passed.

The runtime evidence uses the normal FIFO runner. The initial endpoint runs are in `%TEMP%/quality-teams-relay-824054b3` and `%TEMP%/quality-teams-relay-4fe5a52c`. Intermediate runs are in `%TEMP%/quality-teams-relay-bisect-<revision>`. The passing fix batch is in `%TEMP%/quality-teams-relay-fix`. The long gaps between these short runs were queue time behind other agents' tests, not test execution or extra retries.

## Scope

All browser runs are hidden and use the real frontend with a fixture host. No native window, desktop input, real provider, real-home write, installation, push or merge was performed. The branch changes one renderer expression and adds one browser suite. It does not run the full browser suite or Go suite.
