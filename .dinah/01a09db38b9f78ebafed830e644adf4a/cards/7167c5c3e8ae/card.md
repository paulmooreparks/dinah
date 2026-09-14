---
title: A card's raw identifier still surfaces when two processes race
column: 5ea2db0272fc
state: ready
severity: minor
priority: later
---
Refusals name cards and states by the short name a person types. One path escapes that: if something deletes a card between the moment this process resolves it and the moment it takes the lock, the refusal falls back to the raw identifier.

No single typed command reaches it, so nobody hit it during the work that fixed every other case, and two reviewers judged it correctly out of scope there. It survives because the short name is resolved before the lock and the refusal is raised after it.

The card decides whether that refusal can carry the resolved name across the lock, or whether a race deserves its own wording that does not pretend to name something the person can act on.
