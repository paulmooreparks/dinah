---
title: A state slug shaped like a card reference shadows the card
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
`ResolveEntity` tries a state slug before it tries a card reference, and the state slug grammar admits a final segment of digits, so a state slugged `sprint-2` takes every entity-shaped reference that a person meant for card 2. Reproduced against a binary built from `a62b6fb`: with that slug written into a state anchor by hand, `dinah attach sprint-2 <file>` attached the file to the state while the person naming it was naming a card, and nothing warned. The commands reading `ResolveEntity` are `attach`, `archive`, and `delete`, so the same shadowing decides where an attachment lands and what a delete destroys. This is unchanged by dinah-141, which surfaced it while resolving the same collision class for the workbench slug.
