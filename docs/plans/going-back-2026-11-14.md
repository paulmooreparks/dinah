# Going back to yesterday's build

The fallback tested on invented state is commit 4fe5a52c. That build opens the new database, but starting it alone can leave Notifications unusable. Your running window may be an earlier build; record its version when keeping it, since an unidentified earlier build was not tested. This correction keeps settings.json and notifications.json before their first change, once, beside the originals. It logs the copy and refuses the write if it cannot keep it. The copies are never overwritten.

## Keep the executable before closing the old window

The fallback is the file used by the window you are running now. Record its full path, PID and version from the running-program status. Where Settings > Updates is available, use the row marked this window. If that page is unavailable, find the window's PID in Task Manager > Details and run the following read-only command, replacing 12345 with that PID:

```powershell
Get-CimInstance Win32_Process -Filter "ProcessId = 12345" | Select-Object ProcessId, ExecutablePath
```

The normal folder is `%USERPROFILE%\.dinah\bin`. Installation renames a previous program to `dinah-desktop.exe.old-<suffix>` and installs the new file at `dinah-desktop.exe`. Older names use a numeric time suffix; new names include a unique final suffix. The newest name may be an intermediate build from today, so select the running window's reported path rather than guessing from the name or size.

While that old window still runs, copy its reported executable to a separate folder and name the copy `dinah-desktop.exe`. Record where you put it. This gives you a normal executable name to start afterwards and keeps the original set-aside file intact. Do not copy the newly installed file over it.

The corrected installer keeps all set-aside executables. No installation automatically deletes them, including the running copy and the newest fallback. Their disk use therefore grows with installations. Earlier installers attempted to delete old copies before placing a build; this protection takes effect when the installer code is updated. Earlier installers that are still running retain their old cleanup code, which is another reason to keep your recovery copy outside bin.

## Steps if the new build misbehaves

1. Close Dinah completely, including its separate document and show windows. Keep the executable copy identified above; use it rather than the current installed shortcut.
2. Open `%USERPROFILE%\.dinah` (or the custom home you normally use). Move any `settings.json.pending` and `notifications.json.pending` files into a separate holding folder. Otherwise they can replay a later write or reject the restored file.
3. Copy `settings.json.before-decisions-2026-11-14` over `settings.json`, and `notifications.json.before-decisions-2026-11-14` over `notifications.json`. Keep the backups themselves. If a backup has `.absent` appended instead, no original existed; remove only that corresponding live JSON file. If neither record exists for a file, stop because its rollback copy has not been prepared.
4. Start `dinah-desktop.exe` in the separate folder where you kept the running window's executable. Messages and replies remain in the existing database; do not restore an older bus database as part of these steps.

The restored copies lose settings and notices changed since they were kept. Yesterday has three background chat places and its old seven-day/500-card retention rules, so today's kept attention behavior is lost. Its plain replies do not close structured Decisions requests. Those requests and answer receipts survive in the extra database tables, but yesterday has no window for them. Restart records survive unchanged; yesterday does not automatically send their carry-on messages. Decisions drafts remain stored but cannot be shown there.

Yesterday ignores `webview2/profiles.json`. It uses its own version-named browser folders. It reuses a folder adopted from that version, but cannot see data in today's separately selected `profile-main`, `profile-documents` or `profile-show` folders. Nothing deletes those folders. Native browser storage continuity remains unproved.

Tests used invented data and the actual old source. Both direct readers and a compiled hidden old window accepted the restored copies. No real state or credential file contents were read or changed, and no real backups were taken during this task. The previous executable used in the synthetic trial was rebuilt from 4fe5a52c. Only names and sizes in the real bin folder were inspected; the live window's exact file must be matched using its reported process path as described above. The lead must install this correction before its first protected writes can create the copies. Detailed evidence is in [the verification record](../review/going-back-evidence-2026-11-14.md).
