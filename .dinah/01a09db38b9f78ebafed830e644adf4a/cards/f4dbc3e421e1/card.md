---
title: The query's workstream vocabulary stops being the identifiers live cards happen to list
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
tier: workhorse
---
dinah-135 ruled that the `workstream` query field is queried by identifier alone and that its legal set is whatever identifiers live cards list, which is honest only while no workstream registry exists. dinah-158 creates that registry, so a workstream that exists and that no card has joined becomes a name the tool should recognise rather than a string referring to nothing. This card carries the rule change into the query in the ordering where dinah-158 lands on trunk before dinah-135, which is the ordering where dinah-158's own implementer has no query to change.

Section 13 of dinah-158's spec states the new rule, names the code and catalog edits it needs, and names the three tests on dinah-135's branch that keep passing after the rule stops being true. This card is closed as absorbed when dinah-135 lands first and dinah-158's implementer carries the change instead.
