---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:56Z
ordinal: 21
note: The expression SortByOrdinal(collection, mount.Anchor, ListIDs(collection)) is written twice at b825059, once in descend where a positional selector is counted and once in containmentMembersOf where the walk draws rows. They agree today, which is what makes AC-6's comparison of the two walks meaningful, and nothing holds them together. A card that changed one and not the other would make the position a reference resolves by and the position a screen prints disagree, which is the class of defect dinah-454 was widened to close. The workbench-only branches of containmentMembersOf, for cards and columns, stay where they are, because those two collections are ordered by their own rules and neither can ever be a collection root.
---
One expression lists a collection's members. bench.MemberIDs is extracted and read by descend, by ResolveReference and by Library.containmentMembersOf's default branch.