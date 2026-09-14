---
title: Session-wide flag names are still eaten from free text
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
Five flag names that apply to every command, among them the ones naming a workbench, a language and an actor, are read as flags anywhere they appear, including in the middle of a card title or a comment. A word matching one of them disappears from what gets saved, and for the ones taking a value the following word goes with it.

A card closed this for every other flag name by reading a flag only at the very end of free text. These five were deliberately left as they are, because a sibling card shipped and tested that behaviour and widening the scope mid-flight would have been careless. The result is that the silent deletion is fixed for most names and still present for these.

The card decides whether these five should follow the same rule as the rest, and what that costs anyone who relies on placing them anywhere.
