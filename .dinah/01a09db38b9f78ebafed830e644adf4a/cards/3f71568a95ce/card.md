---
title: The principles sketch describes a listing the tool stopped producing
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
`docs/specs/dinah-182-principles-ux-sketch.md` lands on the trunk with dinah-182 and carries two statements that are no longer true.

It says `guide.Topics()` sorts the embedded filenames, so `principles` lands between `getting-started` and `query`. Topics has read a declared reading order since dinah-164, and `principles` sits after `verbs` because that order places it there. The sketch's own drawing of the listing shows the alphabetical result rather than the shipped one.

It also says the new title grows the second column by four characters. The title is one character longer than the widest that stood there when the sketch was drawn, and the regeneration widened the rule by exactly one. Agent Design Review caught this at the time and recorded the correction in OQ-1's note, so the operator ruled on a corrected drawing; the sketch itself was never edited.

## What to decide first, because it is not obviously a fix

A UX sketch is a spec artifact, drawn to let the operator rule on a shape before the thing existed. It is dated by construction, and correcting it after the fact turns it into a description of what shipped, which is a different document with a different job. Two honest answers, and this card should pick one rather than splitting the difference.

Correct the two statements and the drawing, and the sketch goes on being a document somebody might read for the shape of the guide. Or stamp it as the sketch it was, with a line at the top naming the commit it was drawn against and saying that where it disagrees with the shipped guide the guide wins, and leave the body alone.

The second is cheaper and more honest about what the file is, and it generalises: every sketch under `docs/specs/` has the same problem the moment its card lands, and there are more of them coming. The first is right only if these documents are meant to be read as current.

Whichever way it goes, the ruling belongs in the answer rather than in this card, because it decides what every future sketch is for.

Found at Agent Code Review on dinah-182.
