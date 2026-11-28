# Contributing

## Before every push

Run the review's guards on the exact commit you are about to push:

```powershell
$env:DINAH_FORBIDDEN_TERMS = '<path to the private forbidden-terms list>'
pwsh scripts/guards.ps1
```

Install the locked JavaScript development tools with `npm ci` in `cmd/dinah-desktop/frontend/e2e` before running the guards. The command runs ESLint over desktop and web scripts, audits the complete npm tree, checks the audit gate's failure cases, and runs every guard listed in `scripts/guards.json`. It fails if any listed test is missing, skipped or failing. `-Skip browser` runs only the Go guards, skipping ESLint, the npm audit and its contract tests as well as the browser suites. Nothing is pushed to master unless the forbidden-terms, product-name and provider-name guards pass on that commit. This holds on the fast path too.

The same folder provides `npm run lint` and `npm run audit:guard` for focused checks. Audit exceptions are version- and path-specific and expire; the current exception and its evidence are in `docs/review/supply-chain.md`. Expiry fails the guard even if npm stops reporting that advisory, so the exception must be reviewed and removed explicitly.

The npm audit queries the registry and requires network access. Offline, the guard fails closed with `npm audit did not complete`; this is expected. Restore registry access before running the complete guard command for a push.

The rules these guards protect are in `docs/design/invariants.md`, and a change is reviewed against that list. A new guard is added to `scripts/guards.json` in the same change that adds its test. A guard that only means something on some operating systems is listed under `onlyOn` with those systems, and on any other system the runner requires it to skip.

## Third-party notices

`THIRD-PARTY-NOTICES.md` lists every open-source component that is built into the programs or served by them, with its licence texts. It is written by a tool from what the programs link and what the page bundles, and is never edited by hand. After adding, removing or updating a Go module or a vendored library, write it again and commit the result:

```powershell
go run ./tools/notices
```

`go test ./tools/notices` fails while the committed file is out of date, and the tool refuses to write when a vendored library has no licence file beside it, since such a library could not be listed.

## The full suites

```powershell
pwsh scripts/go-gate.ps1 ./... -count=1
cd cmd/dinah-desktop/frontend/e2e; npm ci; pwsh ./run.ps1
```

Both take the machine-wide test lock, so runs started at the same time queue rather than overlap. The browser suites are headless and use Edge, and they are tested on Windows only.

## Writing browser checks

Dinah draws transcripts, folds, replayed history and agent events a few at a time, one frame's budget after another, so a count taken as soon as the first element appears can catch a list half drawn. A check that counts elements uses `settled` or `settledCount` from `e2e/lib/settled.js`, which waits for the expected count, pauses, counts again and, on failure, prints each match's text, tag, enclosing row or view and whether it is shown. A check that something appears once, or not at all, is a count too and goes through the same helper.

The helper counts hidden matches as well as visible ones, and that default stays. A hidden copy of a line can be a real defect, as the pinned-message strip's was in November 2026, and text that is laid out but invisible is still read by a screen reader. A page that keeps a hidden copy on purpose, such as a measuring clone, a closed menu holding the same words or a reserve badge, is handled by narrowing that suite's locator, for example with `:visible`, and never by changing the helper's default.

A check waits for a stated condition and never for a length of time, so no new `waitForTimeout` or hand-written sleep is added. A check that expects a change waits for that change with `t.until(read, pass)` from `e2e/lib/page.js` or with Playwright's own waits. A check that something did not change first waits on `t.settled()`, which resolves when the command registry has made any waiting redraw (`dinahCommands.idle()`) and the render queue has nothing left that it can draw (`dinahRender.idle()`). Both promises are the product's own and are covered by the `render-scheduler` and `settled-signal` suites. A check about the passing of time drives it with Playwright's clock. The older suites still hold fixed waits, and they are being moved to these waits, the suites with the most waits first.

A suite run directly with `node` takes the test lock itself. Launches within one process share one lease, which ends when each launch's browser closes, and a lease held for more than 15 minutes is released and its suite stopped, so a stuck suite cannot hold every other run. `run.ps1` holds the lock once for its whole batch.

## CI

The Build workflow runs the guards, the full Go suite and the full browser suite as required jobs on Windows, the Go guards again on Linux, and the vulnerability scans on every published platform. It runs on every push to master, on every pull request and when started by hand on a branch. The browser suites are divided between eight runners with `run.ps1 -Shard i/8`, balanced by the times in `e2e/suite-times.json`. A push to master never cancels the build of an earlier commit, and "latest" only moves forward. Building and publication wait for all of them, and `TestEveryPublishedPlatformHasABlockingSourceScan` fails if the workflow stops waiting for any of them. CI has no copy of the private terms list, so there the forbidden-terms guard reports itself skipped, and the local run before a push is what enforces it.

The Stress workflow (`.github/workflows/stress.yml`) is started by hand with a test filter and a number of minutes. It repeats the chosen browser suites or Go tests on a hosted runner for that long and fails if any round failed, which is how a failure that nobody could reproduce is either reproduced or closed.
