# Retained attention verification

The candidate is on `frontend/attention-kept`, based on `92be78f0`. It has not been pushed, installed or merged.

## Cause and correction

The bus watcher dismissed mention cards when deliveries became read. The group conversation reader marks deliveries read during refresh, including restored background groups, so a card could disappear without the person deliberately opening it. Permission questions had another transient lifetime: the page removed their cards when their provider stopped. The Windows notifier uses a tray balloon and taskbar flashing; it does not supply Dinah with a retained notification list. I traced those source paths without observing Paul's original banner, so I cannot identify its exact wording from a live capture.

The candidate saves permission attention before publishing its actionable request, retains mention and relay attention independently of delivery reads, and groups saved identities into one fixed-height card per source chat. The card shows the latest first line and timestamp, with a request count and navigation to the group message, direct bus thread or permission chat. Live permission response controls remain beside the chat's composer. Opening or dismissing a summary acknowledges its captured identities; a later arrival survives that completion. Actual bus replies and successfully claimed permission answers dismiss the corresponding request. Cancellation does not count as an answer.

The pane appends cards in arrival order instead of re-sorting existing cards, reserves its empty-state space and scrolls internally. Clear All dismisses a snapshot rather than a changing store. Active attention no longer expires with ordinary reports. The existing 32 MiB store ceiling still fails a write rather than evicting attention. On migration from the old fixed mention cursor, unread requests are imported and existing cards remain; earlier lost read cards cannot safely be reconstructed after their dismissal records have expired.

The traced native call sites are permission questions, bus mentions, failed team relays and team budget reports. The first three now have retained attention; budget reports already have saved report cards and do not ask for the person. Broker-owned human-request controls retain their existing response lifecycle.

## Verification

Seven affected headless suites passed: `app-ui3`, `attention-kept`, `mention-notification`, `notification-severity`, `notifications-pane`, `notifications-persistence` and `notifications-popover`. Their output is in `%TEMP%/attention-kept-final-20261114`, with an empty `failures.txt`. The initial five-suite pass had two failures because the old assertions required reordering; the fixed arrival-order implementation and revised explicit order contracts passed. That earlier output remains in `%TEMP%/attention-kept-20261114`.

The subsequent navigation checks passed `attention-kept` and `mention-notification` in eight seconds. After adding delayed-refresh protection, `attention-kept`, `mention-notification` and `notifications-persistence` passed in thirteen seconds, in `%TEMP%/attention-kept-refresh-20261114`. Coverage includes grouped counts and duplicate identities, first-line display, fixed card/window/filter rectangles, keyboard focus, restored permission history, dismissal failure, actual group navigation, independent opening, new arrivals during navigation/dismissal and a delayed stored snapshot arriving after a live event.

Focused race tests passed in the desktop, bus and sessionhost packages. The final reported package times were 6.029 s, 2.894 s and 1.118 s. The command selected permission attention, age/capacity retention, mentions, read-before-capture, cursor migration, existing notification/report persistence, host-notice settings and every `TestPermissions` case. The Windows linker printed `.rsrc merge failure: multiple non-default manifests`; the test command exited zero and all three packages reported `ok`. That diagnostic is retained here rather than counted as a silent clean build.

The page-method inventory and exit-class guards also passed in 0.204 s. ESLint passed for the two changed production page scripts, and `git diff --check` passed. New attention guards are registered in `scripts/guards.json`. The full test matrix was not run.

All tests used temporary state, fake native notifiers or headless browser bridges. No native notification, installed application or desktop interaction was exercised. There was no push, install, restart or live-state edit. Dinah Team Lead should review this isolated branch before integrating it.
