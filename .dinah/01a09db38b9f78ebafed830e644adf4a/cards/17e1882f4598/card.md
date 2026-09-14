---
title: The health check blames the wrong column when a column is appended past a done column
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
`dinah check` reports a two-column ordering problem against the wrong end of the relation. When a column is appended to the sequence past a done column, the check accuses the done column, which nobody touched, rather than the newcomer that was appended. An operator reading the finding is sent to a column with nothing wrong with it, and the finding never mentions the column that actually caused it.

Two agents reproduced this independently while working dinah-316, and it was found because a reshape that refused part way through left a workbench in exactly that shape. It is not a defect in reshape. Any command that appends a column past a done column provokes it, and a hand edit does too. The reviewer that confirmed it built the case from the command line in a throwaway workbench with no test hook, so the reproduction does not depend on anything dinah-316 added.

The fix is about attribution rather than detection. The check correctly notices that two columns are in the wrong order relative to each other; what it gets wrong is which of the two to name. Work out what the check should say when the relation is between a column that has always been there and one that has just arrived, and note that "the later one is the culprit" is a rule to argue for rather than assume, since a column can also be moved earlier.

Worth reading dinah-316's third review comment before specifying, since it records the exact shape that produced it and what the operator saw.
