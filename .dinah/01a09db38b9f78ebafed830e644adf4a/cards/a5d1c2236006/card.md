---
title: Criteria that pin a refusal build their expected sentence from the catalog they are checking
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
Three acceptance criteria on dinah-409 assert what a caller reads when a refusal fires, and they build the sentence they expect out of the same message catalog they are checking. So they catch the wrong message being selected and they cannot catch a badly worded one: if the catalog entry is wrong, both sides of the comparison are wrong together and the check passes.

That is the same self-cancelling shape dinah-413 hit from a different direction, where a test compared a printed page against the list the page was generated from, so a planted extra row landed on both sides and the comparison passed. There it was closed by holding the page against a frozen copy that nothing in the tree derives. Here nothing closes it yet.

It matters because of what it let through. dinah-409's whole reason for a second review round was a refusal whose English had been written for a different command, telling a caller to run the block command when they had asked to raise a tier. An agent following that sentence would have blocked the card instead. The criterion covering that case checked only which refusal fired and never read the sentence, which is why nothing caught it; the criteria were then rewritten to read the sentence, and three of them were rewritten into this shape instead.

A fourth gap sits beside it: nothing on that card pins what a successful raise prints. The failure paths are now asserted and the success path is not.

What this card has to settle. Whether a frozen expectation is right here, as it was for the help page, or whether a message meant to be translated needs something else, since a frozen English sentence in a test is a thing translators cannot see and may quietly contradict. Ask what the equivalent of the frozen copy is for a catalog entry, and whether the answer differs for the English source and for a translation.

Note the constraint the board already carries: no fluent reader of German or Hindi works on this project, so a check of this kind can establish that a sentence is the one intended and cannot establish that it reads naturally. Scope it to the first, honestly, rather than implying the second.

Read dinah-413's guard and dinah-409's later review comments before specifying, because both halves of the problem have already been argued out on those cards and the useful work here is joining them rather than rediscovering either.
