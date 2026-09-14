---
title: A stale prefix warns you on one verb and is silent on every other that resolves a card
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
A card reference carries a prefix, and a reference whose prefix no longer matches still resolves. `dinah move` warns you about that. `dinah comment`, `dinah file`, `dinah attach` and `dinah set` say nothing at all, confirmed by running each rather than by reading.

The cause is structural rather than a missing line in four places: the warning is set inside the one path the contract verbs take, and every other verb that resolves a card takes a different path. So the set is not four commands, it is every verb resolving a card outside that path, and it will grow by one each time somebody adds a verb without noticing which path they took.

Found on dinah-471 while establishing which spellings the tool accepts. Its spec author recorded it rather than widening that card's diff, and its reviewer confirmed the behaviour by running and ruled it wanted a card of its own, filed as the class rather than the sighting.

What the card should settle, none of it the operator's to rule on:

Which verbs resolve a card, and which of them can warn today. Enumerate by tracing the call paths rather than by reading the resolver: dinah-471 counted eleven accepting places by reading and nineteen by tracing, and this board has now made that same error four times, each time with reading supporting the wrong number all the way. Say how you enumerated and what would make your number wrong.

Whether the warning belongs where it is. A warning attached to one path, which a verb silently opts out of by taking another, is a warning that will keep going missing. Moving it to where a card is resolved rather than to where a verb runs is the obvious shape, and the obvious shape is worth checking against the reason the current placement was chosen.

What a reader sees when they use a stale prefix on a verb that has never warned. It is not a refusal and must not become one: the reference resolves and the work happens. So this is about telling somebody their spelling has drifted, which means deciding whether that is worth saying every time, once, or only when the correct spelling differs in a way that matters.

Whether anything guards it afterwards. A warning is exactly the kind of thing that goes missing without failing a test, and this workstream has closed several checks that could not fail. Assert how many verbs the sweep examined, because a sweep that reads nothing reports success.

Related: dinah-471 is where it was found and deliberately does not fix it. dinah-470 landed one declaration of which address kinds each command accepts, which is the shape to reach for if a per-verb table turns out to be needed.
