# Settings Updates carry

The branch `frontend/settings-updates` carries the generic Settings page from its implementation review. It was created from master at `27920719`. This branch is prepared for review and has no release authorization.

The page uses the existing `updates` product service through `CallService` and the existing package operations. It contains no release-selection, executable-placement, process-scanning or settings-file mutation logic. No App method or pageMethods entry is added. Service behavior remains the responsibility of the build that provides it.

## Capability boundary

The Updates tab starts hidden. `ProductServices` must return `updates` before it becomes available. A missing method, rejected discovery or a list without that service leaves it hidden. Settings excludes hidden sections when restoring a section address and ignores hidden-tab clicks. Ordinary Dinah therefore retains its existing eight visible Settings sections.

The service suite covers an empty service list, an unrelated service, rejected discovery, an absent discovery method and the supplied Updates service. The supplied-service fixture exercises the page without a native executable or live source. Existing state suites cover loading, current and available releases, installation progress, installed state, unavailable builds, errors, packages, repair and missing service methods.

## Shared changes

The carry includes Settings markup and styles, the page controller, the package installer's optional leave-running behavior and disabled-state handling in Settings menu commands. Existing package entry points keep their behavior. This page's package route leaves agents running after the existing preview and token-checked installation. Both repair and release actions reject repeated activation while pending.

The page reserves row and section dimensions and reconciles process, package and repair rows without removing existing rows. Text from services is rendered as text. Restart status displays active chats and document writes, and its button opens the existing Notifications workflow.

The browser suites and fixture are generic. An optional environment-supplied script lets a receiving build test its existing update entry point without naming it in shared source. The public fixture uses a second caller of the same synthetic service when that script is absent. `scripts/guards.json` adds mandatory suite names only; it adds no exception.

## Verification

The selected browser batch uses Playwright with explicit `headless: true`, the shared FIFO and synthetic bridge responses. It performs no native-window, desktop-input, real installation, restart or external-service operation. The command runs from `cmd/dinah-desktop/frontend/e2e` with `NODE_PATH` pointing to the existing development dependency tree:

```powershell
./run.ps1 -Only settings-updates-service,settings-updates-loading,settings-updates-current,settings-updates-available,settings-updates-installing,settings-updates-installed,settings-updates-unavailable,settings-updates-errors,settings-updates-packages,settings-updates-repair,settings-updates-missing-services,settings-ui,packages,menus-through-updates,shell-settings -Jobs 1 -Out C:/Users/paul/AppData/Local/Temp/settings-updates-upstream-20261114
```

All fifteen selected suites passed in 144 seconds, one at a time, after waiting for the shared FIFO. The retained `failures.txt` is empty. Per-state call ledgers and synthetic screenshots are in the output directory named above. Focused ESLint and `git diff --check` also passed. There was no failing browser run for this carry. The initial transfer attempt failed on patch line-ending conversion before applying any file; using Git's direct patch output resolved that transfer problem.

A direct scan found no private product or customer names in the carried files. The private forbidden-terms list is not configured in this environment, so its guard has not been claimed as passed. No Go code changed and no Go suite was run for this carry.

These browser fixtures do not establish live executable replacement, actual agent-settings preservation, reconnect behavior or publication draining. The current implementation branch remains under its separate review. This carry has not been pushed, merged or installed.
