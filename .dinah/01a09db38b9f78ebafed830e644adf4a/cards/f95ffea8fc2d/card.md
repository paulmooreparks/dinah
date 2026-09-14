---
title: One message serves two unrelated refusals, and it has never been true of the second
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The message the checker prints for an entity with no stored ordinal is reused for a second, unrelated case: a card missing its reference number. The two have nothing in common. An entity's ordinal is a sort key, so a missing one leaves the entity's place to be worked out, and the message explaining that makes sense. A card's number is its name rather than a position in a list, so nothing about ordering applies, and the message has misdescribed that case for as long as it has been shared.

This was found while working dinah-262, which corrected the message for the case it does fit. That card left this alone deliberately and recorded the exclusion rather than smuggling a wider fix in, because giving the second case a message of its own reaches all eight translation catalogs, two of which carry real translations rather than English placeholders and need a translator's judgement rather than a copy.

What is wanted is a second message, written for what a missing card number actually is, landed across every catalog with the two real translations rewritten in their own language rather than filled with English. While the catalogs are open, check whether any other message is shared between two callers that do not describe the same situation, and establish that by reading each message's callers rather than by searching for reuse of a key, because a shared key is easy to find and a shared meaning that has quietly diverged is not.

One caution drawn from this week. Every count of affected sites produced on this board recently has been short, including the count for the very message this card is about, where a review said six catalogs and the true number was eight, and the two it missed were the only two that carried real translations. Derive the set rather than inheriting it.
