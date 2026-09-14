---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:44Z
ordinal: 22
note: "DOC-CHG-1 (docs/spec/core-profile.md:172) says a published changelog entry must not be edited or removed, so the parenthetical at :1596 calling `number` a field the interchange form carries stays byte-identical where it stands inside the published 2.0 entry. The new entry names the entry it corrects and states the fact: the interchange form carries columns and no cards at all. Round 1 stated this rule and then instructed editing two sentences at :2015 and :2026, which sit inside the published 0.12 entry. They are not current-revision sites. The document names its current revision in exactly five present-tense places, which TestAllOfTheProfilesRevisionStatementsAgree (internal/profile/amendment_test.go:174) enumerates: the header, section 2, section 2.1's conformance-claim example, section 5.7's interchange-form example, and the head of section 12. Every other occurrence of a revision number, including :2015, :2026, :2058 and :2064, names a past event and is historical record. AC-14 drives the whole rule by diff."
---
The profile changelog's false claim is corrected by a new 0.14 entry, not by editing the 2.0 entry, and nothing else inside a published entry is edited either.