---
kind: decision
state: resolved
ts: 2026-09-14T02:16:53Z
ordinal: 9
note: Section 2.2 makes the extracted statement list the unit of change, and prose carrying no keyword on a statement line is not in it. DOC-VER-1 and section 2.2 make an editorial change a patch increment, and the published version carries a major and a minor only, so a patch has no number to move. DOC-CHG-2 asks an entry for the identifiers it affects, and this affects none. dinah-201 ruled the same question the same way, and internal/profile/amendment_test.go enforces the ruling with publishedStatements=127, the section 2 version sentence, and publishedChangelogEntries=4, all three untouched here.
---
The profile's version stays dinah-core 0.4 and section 12 gains no changelog entry.