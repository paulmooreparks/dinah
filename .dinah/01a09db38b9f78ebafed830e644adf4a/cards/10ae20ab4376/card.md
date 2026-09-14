---
title: The profile's pointer guard checks only the first path in a section
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
`TestTheCardSectionPointsAtADocumentTheRepositoryCarries` in `internal/profile/extract_test.go` extracts a document path out of section 5.3's own prose and stats what it extracted, which is the right shape: it reads the document rather than asserting a constant that agrees with itself. It matches only the first Markdown path in the section. A second reference added alongside the real one would never be checked, so a bogus path could sit in that section indefinitely with the guard green.

Found on 2026-08-27 by the Test pass on dinah-198, which reproduced all five known mutations against the guard and then went looking for ways it could pass when it should not. An emptied section and a malformed path both fail correctly. This one does not.

It is not a defect in dinah-198 and that card was not failed for it. Section 5.3 carries exactly one pointer today, so the guard covers everything there is to cover, and the criterion it was written against is met. This card exists because the gap becomes real the moment that section grows a second citation, and the person adding that citation is the least likely to know the guard stops at the first one.

What to weigh rather than assume. Whether the guard should check every path in the section, which is the obvious answer and costs little. Whether the same shape is repeated elsewhere in `internal/profile`, since the guards there share an idiom and one of them stopping at the first match suggests reading the others. And whether the fix belongs in the extraction helper rather than in the one test, so a future guard written to the same pattern inherits the correct behaviour instead of the current one.

Worth reading first: dinah-198's AC-5 and the five mutations recorded in its note, which are the arming this guard already has and which any change here must keep passing.
