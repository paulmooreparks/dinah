---
title: A tree whose subject is an act rather than a card
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
A standup rollup wants journal events grouped by day and then by actor, and dinah-151's tree cannot answer it however many axes it gains. Its subjects are cards, so every count it reports counts cards, and a rollup of events counts events. dinah-151 does group on what a card did, since a card is drawn under every actor who acted on it, and grouping by time is the one thing it refuses, because an instant is a different value on every act and no bucket granularity has been chosen. This card asks whether a third producer exists whose `Tree.Subject` is an act, what a node of it is, and what a day, a week, or a month means as a group key.
