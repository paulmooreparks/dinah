# Going back to yesterday's build

The fallback is commit 4fe5a52c. Its database access still works, but starting that executable alone can leave Notifications unusable. This correction keeps settings.json and notifications.json before their first change, once, beside the originals. It logs the copy and refuses the write if it cannot keep it. The copies are never overwritten.

## Steps if the new build misbehaves

1. Close Dinah completely, including its separate document and show windows. Keep the previous executable ready; use the saved build from 4fe5a52c rather than the current installed shortcut.
2. Open `%USERPROFILE%\.dinah` (or the custom home you normally use). Move any `settings.json.pending` and `notifications.json.pending` files into a separate holding folder. Otherwise they can replay a later write or reject the restored file.
3. Copy `settings.json.before-decisions-2026-11-14` over `settings.json`, and `notifications.json.before-decisions-2026-11-14` over `notifications.json`. Keep the backups themselves. If a backup has `.absent` appended instead, no original existed; remove only that corresponding live JSON file. If neither record exists for a file, stop because its rollback copy has not been prepared.
4. Start the previous executable. Messages and replies remain in the existing database; do not restore an older bus database as part of these steps.

The restored copies lose settings and notices changed since they were kept. Yesterday has three background chat places and its old seven-day/500-card retention rules, so today's kept attention behavior is lost. Its plain replies do not close structured Decisions requests. Those requests and answer receipts survive in the extra database tables, but yesterday has no window for them. Restart records survive unchanged; yesterday does not automatically send their carry-on messages. Decisions drafts remain stored but cannot be shown there.

Yesterday ignores `webview2/profiles.json`. It uses its own version-named browser folders. It reuses a folder adopted from that version, but cannot see data in today's separately selected `profile-main`, `profile-documents` or `profile-show` folders. Nothing deletes those folders. Native browser storage continuity remains unproved.

Tests used invented data and the actual old source. Both direct readers and a compiled hidden old window accepted the restored copies. No real home or credential file was read or changed, and no real backups were taken during this task. The tested previous executable was rebuilt for the trial; this note does not identify an installed backup executable. The lead must install this correction before its first protected writes can create the copies. Detailed evidence is in [the verification record](../review/going-back-evidence-2026-11-14.md).
