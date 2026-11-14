# Cross-version rollback evidence

Yesterday's build, 4fe5a52c, opens today's bus database and loads the ordinary settings. It is not a lossless fallback on the same home. Tests against the actual old source confirm the following limits on invented data written by current master 4cd9b954.

| Stored state | What yesterday's build does |
| --- | --- |
| Bus database | It opens the database, reads messages and queues replies. Today's three Decisions tables and added index are additive. The old program leaves those tables intact. |
| Decisions | It reads a structured request as ordinary message text, but has no Decisions window or answer operation. A plain reply does not update the structured answers or close the request. Returning to today's build still shows the unanswered question. Existing answer receipts survive. |
| Notifications | If notifications.json contains a permission attention card or a restart recovery card, yesterday rejects the whole file as an invalid durable notification. Listing and adding notifications both fail. Startup still succeeds, and the file remains intact for today's build. |
| Kept attention | With only older, recognized card kinds, yesterday applies its seven-day and 500-card limits. Reading hides older/overflow cards; its next notification write removes them from the saved file. Our fixture had 504 cards; the old list exposed 500. |
| Settings | An ordinary settings save succeeds but removes backgroundChatLimit and attentionReadSafe, which yesterday does not know. Yesterday uses three background places. Returning to today's build uses its default of eight and the older attention migration path. Known settings survive. |
| Restart records | Yesterday ignores restart-resume.json and restart-awaiting.json. Both remain unchanged, but it sends no automatic carry-on message from those records. Today's build can still read the records afterwards. |
| Browser profiles | Yesterday ignores webview2/profiles.json and selects its own version-named folders. It sees an adopted profile when that is its old version folder. If today's selected folder is profile-main, profile-documents or profile-show, yesterday selects different folders. The selected folder and record are not deleted. Native browser storage itself is not proved by marker files. |
| Decisions drafts and layout | The existing version-one ui-state.json reader accepts and preserves the new Decisions draft key through an old API rewrite. Yesterday cannot display the Decisions window. Browser layout restoration across the missing applet has not been checked. |

There is no tested guarantee that replacing only the executable restores yesterday's full behavior. The notification incompatibility is the main obstacle to that fallback. These results describe the currently shipped formats; this branch changes no production writer or migration.

## Evidence

The trial is on engine/going-back. scripts/going-back-trial.ps1 creates synthetic homes and runs current writers, a test compiled against unmodified production source at 4fe5a52c, and current readers again. Only the injected test file is added temporarily to the separate old checkout and removed afterwards. The current and old test commands passed. The compiled hidden-window observations and backup correction are recorded below.

The inventory also checked today's document-write admission, diagrams and source-history changes. They add no new draft store schema or persisted history encoding. Broadcast operations and previews are in the Decisions tables; waiting-chat notices are ordinary bus host results. No bus column was removed or retyped, and no schema version rejects the old build.

No real home folder or credential file was opened, copied or changed. All messages, settings, profile markers and operation records came from the fixture. Evidence stays under TEMP, and no private message text is included here.

## Backup correction

Source commit fe9b0219 adds one-time copies named settings.json.before-decisions-2026-11-14 and notifications.json.before-decisions-2026-11-14. Both writers use statefile.WriteValidatedKeeping. The copy is made from the previous bytes after pending-write recovery and validation but before the replacement transaction is published, under the same process writer mutex. It uses the existing complete-file Publish mechanism, which refuses an existing destination. A copy failure leaves the live file unchanged. A missing original gets a separate .absent marker, preventing a later new-format file from being mistaken for the original. The caller logs creation even if the following replacement fails.

Tests verify byte identity, one-time creation, concurrent calls, refusal when the copy cannot be made, absence recording, and both actual application writer paths. The old-source test restores the actual copies into another synthetic home and verifies startup, settings, Notifications reading and an ordinary notification write. It also constructs conflicting pending transactions, verifies that they reject startup, moves them aside and verifies restoration. The new reader checks the database and unchanged incompatible notification file after the old run.

## Executable observations

The old desktop was compiled from 4fe5a52c with desktop,production tags, -buildvcs=false and a fixture version of rollback-old. Its SHA-256 is ece690ecfae8e427ddce9e9bfb290ab565c437688efd3d1d5837b416f4c2d06a. This is a test rebuild, not a located copy of yesterday's installed executable.

The hidden old program used the native synthetic home and redirected USERPROFILE, HOME, APPDATA, LOCALAPPDATA and provider configuration roots. Headless Edge used a separate temporary browser profile. No real account configuration or credential source was copied. The old window served its entry address and page with zero page errors. It listed the synthetic request in its inbox and returned the stored Decisions draft. With the new notification kinds still present, its Notifications call failed as predicted. Its normal page startup removed the unknown settings fields. The restart files, profile map and notification file were byte-identical at the observation checkpoint.

The same compiled executable was then started hidden after the fixture restored the two protected copies. Startup, inbox and draft access passed again, and Notifications returned an empty list without error. Both runs terminated their own fixture process. Native window visibility was not visually inspected; the standard hidden hook was used. No shared desktop input or model agent was used.

The first executable attempt failed because redirecting the browser's Windows profile environment prevented Edge from opening its debugging pipe. The native child had reached its entry file. The successful harness keeps the native child isolated while allowing Playwright to launch Edge with its own temporary user-data directory. That failed attempt is retained in the -02 output folder. The -01 attempt failed before tests because offline checksum settings prevented the Go toolchain launcher from verifying itself; the final runner uses the installed toolchain directly and then disables module downloads.

## Commands and scope

The cross-version runner is used through the shared FIFO:

```powershell
scripts/with-test-lock.ps1 pwsh -NoProfile -File scripts/going-back-trial.ps1 -YesterdayTree <separate-4fe5a52c-checkout> -Out <new-folder-under-TEMP>
```

The executable check uses pinned Playwright Core and installed Edge. Set NODE_PATH to that test-only driver and run:

```powershell
scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/going-back-hidden.cjs <old-desktop.exe> <fixture-root>
scripts/with-test-lock.ps1 node cmd/dinah-desktop/frontend/e2e/going-back-hidden.cjs <old-desktop.exe> <fixture-root> restored
```

The full touched-package command passed under FIFO. internal/statefile passed in 10.000 seconds, cmd/dinah-desktop passed in 172.071 seconds, and the product subpackage has no test files:

```powershell
scripts/go-gate.ps1 ./internal/statefile/... ./cmd/dinah-desktop/... -count=1
```

Raw cross-version logs and the successful executable receipts are under %TEMP%/dinah-going-back-20261114-03. The final -04 run, including the pending-transaction regression and restored old reader/writer, exited zero. Its current-writer test passed in 1.270 seconds, old-reader/writer test in 0.412 seconds, and current-reader check in 0.216 seconds. No full browser suite, push, merge or installation occurred. The tested schema is master 4cd9b954 plus this branch's backup correction; later independent schema changes are outside this result.
