---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:47Z
ordinal: 7
note: "Rewritten at round 4 only in its subject and in one word: the driven thing is the table entry rather than round 3's `archiveCards`, and the count the copy names is the number of rows that resolved to a card context rather than the number that survived a filter, because §1 removes the filter and `askArchiveConfirmation` is handed the resolved list by `runBulk`.\n\nWhat it compares at the moment it runs: the two strings recorded by a fake host against the strings a localizer built over the English catalogue produces for those keys and values. A wrong key, a missing fill, or a second confirmation call all fail. What this cannot check is whether the sentence is true, and that is a human read at Test against the two English entries and their context notes; a test comparing prose against prose would prove only that somebody maintained the copy twice."
---
The Archive entry's `invoke` calls `confirmDestructive` exactly once whatever the row count, asking with `dialog.archive.confirm.one` filled with the reference when one row resolved to a card context and `dialog.archive.confirm.many` filled with the count when more than one did, and with `dialog.archive.action` as the affirmative label in both.