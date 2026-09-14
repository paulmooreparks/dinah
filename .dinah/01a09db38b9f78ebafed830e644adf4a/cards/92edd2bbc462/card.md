---
title: Four surfaces each spell out how a position is computed
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Four call sites compute a member's position in a collection, and each spells the expression out rather than calling a shared helper. They agree today because dinah-186's second repair made them agree, and a code review checked all four rather than the two that card's account mentioned.

Two computations that agreed today are what produced dinah-186's blocker in the first place, so agreement by inspection is the weaker property. Inside the resolver it is already one function, and the arm answering a positional reference and the arm printing the numbers in the ambiguous-name refusal index the same list, so those two cannot drift. The remaining exposure is across the four surfaces.

One apparent duplicate is load-bearing and must survive any consolidation. The display re-lists the collection instead of reusing the loop index it already holds, because the lister skips an attachment whose anchor will not read and the resolver does not skip it. Reusing the loop index would reintroduce the same class of defect on a card with a torn attachment directory. So a shared helper has to take the resolver's view of the collection rather than the lister's.

Not urgent: nothing is wrong today. The value is that a fifth surface, or a change to how sorting works, cannot silently disagree with the resolver.
