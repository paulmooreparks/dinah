# Contributing

## Before every push

Run the review's guards on the exact commit you are about to push:

```powershell
$env:DINAH_FORBIDDEN_TERMS = '<path to the private forbidden-terms list>'
pwsh scripts/guards.ps1
```

The command runs every guard listed in `scripts/guards.json`: the static checks, the security and stability tests, and the browser suites that protect the review's fixes. It fails if any listed test is missing, skipped or failing. `-Skip browser` runs only the Go guards. Nothing is pushed to master unless the forbidden-terms, product-name and provider-name guards pass on that commit. This holds on the fast path too.

The rules these guards protect are in `docs/design/invariants.md`, and a change is reviewed against that list. A new guard is added to `scripts/guards.json` in the same change that adds its test. A guard that only means something on some operating systems is listed under `onlyOn` with those systems, and on any other system the runner requires it to skip.

## The full suites

```powershell
pwsh scripts/go-gate.ps1 ./... -count=1
cd cmd/dinah-desktop/frontend/e2e; npm ci; pwsh ./run.ps1
```

Both take the machine-wide test lock, so runs started at the same time queue rather than overlap. The browser suites are headless and use Edge, and they are tested on Windows only.

## CI

The Build workflow runs the guards, the full Go suite and the full browser suite as required jobs on Windows, together with the vulnerability scans on every published platform. Building and publication wait for all of them, and `TestEveryPublishedPlatformHasABlockingSourceScan` fails if the workflow stops waiting for any of them. CI has no copy of the private terms list, so there the forbidden-terms guard reports itself skipped, and the local run before a push is what enforces it.
