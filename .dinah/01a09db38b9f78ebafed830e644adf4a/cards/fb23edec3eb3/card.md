---
title: An empty collection is not an unknown path
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Asking to show a card's comments when it has none refuses as though the path were wrong, so a reader who typed a correct reference is told they typed a bad one. The same holds for a card's attachments and its checklist. Nothing is malformed and nothing is missing; the collection is simply empty, and saying so is both true and more useful than a refusal. This also affects the exit code, since a refusal ends at two while an empty listing should end at zero, so a script asking whether a card has comments currently cannot tell an empty card from a mistyped reference.
