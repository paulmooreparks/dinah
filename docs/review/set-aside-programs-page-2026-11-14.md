# Set-aside programs page

The page is ready on `frontend/set-aside-programs` in `.worktrees/set-aside-programs`, based on master `bc273498`. The UI source is `b919195531899c99a12885d348393ce2d3dd5af2`; a following test-only commit makes the narrow capture use the existing maximized window mode. The [page design](../design/set-aside-programs-page.md) records the presentation contract.

This branch contains presentation and browser tests only. It matches Engine's immutable service implementation `b49f14361e58743fe342131775839d062278d58c`, also present at its verification tip `6bc44d2d7ad145b3ebe6a282ddaa5053f60d0c12`. Dinah Team Lead should review the two branches together, with Quality's separate service evidence, before combining them. No backend files or pageMethods rows were changed here; Engine owns those additions.

## What the page does

Settings has a Set-aside Programs section when both host methods exist. The section is independent of Updates and accepts any program inventory supplied by the product profile. It shows each copy's name, logical size, date, protection reason and running process IDs when supplied. Inspection warnings remain visible. Running, fallback and unsupported entries cannot be selected.

Only selected opaque IDs are submitted, with a maximum of one hundred. Confirmation names those copies and their logical total. Cancellation leaves selection intact and sends no removal. Selection and refresh are gated while confirmation or removal is pending. Changed identities never inherit selection; refreshed protection removes a selection.

Every result is shown, including a mixed batch of removed and refused copies. Lost, rejected or malformed responses remain unconfirmed, clear selection and trigger a list read without automatically retrying removal. A later attempt needs a new selection and confirmation. The page promises logical byte totals rather than physical space recovery.

The list, status, warnings, selected total and results have fixed-height regions. New identities append, and missing identities retain disabled rows in place. The fixed-geometry assertion compares the same regions and earlier identities before and after an arrival. The section reuses the existing Settings and PUDL markup and styles. No product-specific code or release logic was added.

## Verification

The final six-suite headless run passed in 45 seconds with exit zero under FIFO at `%TEMP%/settings-set-aside-final-20261114`. Its failure record is empty. The suites were settings-set-aside-list, settings-set-aside-remove, settings-set-aside-errors, settings-ui, shell-settings and settings-updates-service. The three new suites are registered as browser guards.

The cases cover profile programs, date and size disclosure, warnings, fallback and running protection, unsupported platforms and older hosts, empty inventory, literal hostile-looking names, geometry on arrival, selection totals, changed identities and new fallback protection, narrow overflow, confirmation cancellation, exact IDs, duplicate-submit gating, mixed refusal, lost and malformed results, malformed inventories, read failures and the hundred-selection limit. The final narrow-window driver then passed alone in nine seconds at `%TEMP%/settings-set-aside-narrow-final-20261114`, with production bytes unchanged. It maximizes Settings before narrowing the viewport and verifies that the window fits.

Earlier development failures remain in their original temporary folders. The first run failed because the fixture referenced a Node variable inside the page; setup cleanup now closes the browser on failure. Two subsequent driver failures came from comparing the insertion's numeric position with old controls, and asking Playwright to click an aria-disabled button. The geometry driver now compares the original identities, and the duplicate-submit check invokes the button programmatically while keeping its gate assertion. No existing Settings assertion was weakened. Those records are at `%TEMP%/settings-set-aside-first-20261114`, `%TEMP%/settings-set-aside-second-20261114` and `%TEMP%/settings-set-aside-geometry-20261114`.

ESLint passed for settings-set-aside.js. Node syntax checks passed for the new fixture and three suites, and diff whitespace checks passed. Desktop, results and narrow screenshots were inspected. Local screenshots and call ledgers are retained in `.diagnostics/set-aside-programs/` in this worktree.

These tests use invented service responses and do not remove real programs. Engine and Quality own the real hidden-process and filesystem checks. No combined live-program UI, full browser run or Go suite was executed here. No real home-folder write, credential read, visible window, desktop input, push, merge or installation occurred. Front End retains ownership for interface findings.
