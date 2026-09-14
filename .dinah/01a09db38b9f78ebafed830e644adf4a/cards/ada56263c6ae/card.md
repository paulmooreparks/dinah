---
title: Two more comments describe the stored ordinal as a position, and one contradicts the function beneath it
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Work on dinah-260 turned up two further places that repeat the belief that card was written to correct, and neither is reachable by the phrase search that found the first three. They were found by searching for the claim instead of the wording, and they are recorded here rather than folded into that card, which had already been through three design reviews on a settled site list.

The first is a contradiction inside one file. A struct field's documentation in the read path says the attachment view's ordinal is the stored ordinal, or the directory-order position on an attachment whose anchor carries none. The function that fills that field says, in its own comment about a hundred and ninety lines further down, that it counts the member's place and never reads the stored ordinal. One of the two is wrong about what the code does, and a reader who consults the field's documentation is told the opposite of what the code beneath it does.

The second is in the entity code, which calls the stored ordinal the entity's one-based position, assigned at write time. That is untrue even at write time once a delete has gapped the collection, because the allocator hands out the highest existing value plus one rather than a count of what is present.

Both statements are the same mistake dinah-260 corrected elsewhere, which is treating a sort key as an address. The first one carries a second problem on top: its clause about directory order on an unmigrated anchor is a claim about pre-migration ordering, and that is the question dinah-262 is deciding. Whichever way that lands, this sentence has to agree with it, so this card should be worked after dinah-262 rather than alongside it.

What is wanted is that each of the two sites says what its code does, and that the field's documentation and the function that fills it stop disagreeing. Establish the true set by reading for the claim rather than searching for a phrase, because that is how these two were missed the first time, and because the last three cards on this board that named a count of stale sites each had the count right and the membership wrong.
