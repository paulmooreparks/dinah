# Contributing

## Before every push

Run the review's guards on the exact commit you are about to push:

```powershell
$env:DINAH_FORBIDDEN_TERMS = '<path to the private forbidden-terms list>'
pwsh scripts/guards.ps1
```

Install the locked JavaScript development tools with `npm ci` in `cmd/dinah-desktop/frontend/e2e` before running the guards. The command runs ESLint over desktop and web scripts, audits the complete npm tree, checks the audit gate's failure cases, and runs every guard listed in `scripts/guards.json`. It fails if any listed test is missing, skipped or failing. `-Skip browser` runs only the Go guards. Nothing is pushed to master unless the forbidden-terms, product-name and provider-name guards pass on that commit. This holds on the fast path too.

The same folder provides `npm run lint` and `npm run audit:guard` for focused checks. Audit exceptions are version- and path-specific and expire; the current exception and its evidence are in `docs/review/supply-chain.md`. Expiry fails the guard even if npm stops reporting that advisory, so the exception must be reviewed and removed explicitly.

The rules these guards protect are in `docs/design/invariants.md`, and a change is reviewed against that list. A new guard is added to `scripts/guards.json` in the same change that adds its test. A guard that only means something on some operating systems is listed under `onlyOn` with those systems, and on any other system the runner requires it to skip.

## The full suites

```powershell
pwsh scripts/go-gate.ps1 ./... -count=1
cd cmd/dinah-desktop/frontend/e2e; npm ci; pwsh ./run.ps1
```

Both take the machine-wide test lock, so runs started at the same time queue rather than overlap. The browser suites are headless and use Edge, and they are tested on Windows only.

## Writing browser checks

Dinah draws transcripts, folds, replayed history and agent events a few at a time, one frame's budget after another, so a count taken as soon as the first element appears can catch a list half drawn. A check that counts elements uses `settled` or `settledCount` from `e2e/lib/settled.js`, which waits for the expected count, pauses, counts again and, on failure, prints each match's text, tag, enclosing row or view and whether it is shown. A check that something appears once, or not at all, is a count too and goes through the same helper.

The helper counts hidden matches as well as visible ones, and that default stays. A hidden copy of a line can be a real defect, as the pinned-message strip's was in November 2026, and text that is laid out but invisible is still read by a screen reader. A page that keeps a hidden copy on purpose, such as a measuring clone, a closed menu holding the same words or a reserve badge, is handled by narrowing that suite's locator, for example with `:visible`, and never by changing the helper's default.

A suite run directly with `node` takes the test lock itself. Launches within one process share one lease, which ends when each launch's browser closes, and a lease held for more than 15 minutes is released and its suite stopped, so a stuck suite cannot hold every other run. `run.ps1` holds the lock once for its whole batch.

## CI

The Build workflow runs the guards, the full Go suite and the full browser suite as required jobs on Windows, the Go guards again on Linux, and the vulnerability scans on every published platform. Building and publication wait for all of them, and `TestEveryPublishedPlatformHasABlockingSourceScan` fails if the workflow stops waiting for any of them. CI has no copy of the private terms list, so there the forbidden-terms guard reports itself skipped, and the local run before a push is what enforces it.
