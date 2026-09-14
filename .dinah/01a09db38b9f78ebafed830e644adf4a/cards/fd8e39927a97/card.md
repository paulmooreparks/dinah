---
title: The translation-staleness document states a width limit that does not exist, and translators are quoting it as a reason
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
The workbench's own translation-staleness document says a printed help table is held to a 44-column width limit, and names a test that supposedly fails a row for exceeding it. Neither is true. The test it names asserts something else: that columns are tight and that an over-wide block falls back to a stacked layout, which is a different rendering rather than a refusal. No such limit is in force.

The German catalog already carried a 51-character cell before any of this week's work touched it, and dinah-413 shipped cells of 53 and 59 characters with CI green throughout. A limit cannot have forced a rendering that exceeds it by fifteen characters.

This is not a documentation nit, because the sentence is being used. dinah-413's implementer wrote a translation record explaining that two German and Hindi renderings read loosely because that width limit forced the terseness. The implementer did not invent the claim; it quoted this document almost word for word, faithfully. The wording changes themselves were described honestly and nothing was hidden. What was wrong was the reason, and the reason is the part of a translation record the whole convention exists to capture: a record exists so a later reader can tell a forced choice from a free one, and this document turns free choices into forced ones for anybody who trusts it.

So the defect propagates. Every translator who reaches for a reason will find this sentence, quote it in good faith, and produce a record that reads as an explanation while explaining nothing. It has done so once already, on the card that found it.

What this card has to establish before it changes anything. What the named test actually asserts, in its own words, since the document's account of it is wrong and a replacement written from the same misreading would be no better. Whether any width constraint exists anywhere in the rendering, and if one does, what it really governs and what happens when a row exceeds it, because the honest answer may be that the layout adapts rather than that anything fails. And whether other passages in the same document rest on the same misreading, since a document that is wrong about one test it cites may be wrong about others.

Then fix the passage to say what is true, and say plainly what a translator should do when a rendering will not fit, since that is the question the false sentence was answering badly.

Found by the second code review on dinah-413, which read the shipped German against the record's stated reason rather than against the English alone. Its review comment carries the character counts and the test's actual assertions.
