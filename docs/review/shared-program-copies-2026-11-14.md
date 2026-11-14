# Shared program-copy card

The candidate is on `frontend/updates-set-aside-carry`, based on master `29c63046`. Source is frozen at `3f6be58c2bc4edbca63fe61a612256ffd03482e2`. The [design note](../design/shared-program-copies.md) describes how the original Settings location and optional Updates slot share the existing card.

The carry includes placement in settings-set-aside.js, the Updates slot, spacing, inventory refresh calls, fixtures and the shared-card suite. Its extra case verifies Settings without an Updates service, eligible selection there, reload and the absence of any Updates operation. Existing service-discovery cases cover absent, unrelated, failed and present services. The guard inventory gains the new suite without a policy exemption. The changed files were checked for private product and customer names; none were found.

Focused Go frontend policy, UTF-8 and release-asset checks passed in 0.483 seconds, exit zero. ESLint passed for the two modified production modules. Node syntax and diff whitespace checks passed. All fifteen touched headless browser suites passed in 73 seconds, exit zero, through FIFO with one browser job. Their output is at `%TEMP%/updates-set-aside-carry-20261114`, with an empty failures.txt. The standalone Settings case passed with no Updates service or Updates operation. The remaining cases verify shared state in both directions, fixed geometry, protection and confirmation, identity-only removal, post-install refresh and narrow layout.

The browser checks use invented service responses. No installed-program journey, live update, actual removal, real-home write, credential read, full suite, visible window, desktop input, push, merge or installation occurred. The implementation adds no host method or removal rule.
