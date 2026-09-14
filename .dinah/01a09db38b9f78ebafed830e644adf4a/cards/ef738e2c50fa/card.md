---
title: A freshly created attachment records no path, though the file was just written
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Adding an attachment writes its payload to disk and then builds the record describing what was added. That record leaves the path empty, even though the code doing the writing knew the path a moment earlier.

Nothing reads it today, so nothing is broken. The reads that publish an attachment's path build their own view from the stored entity rather than from this record, so a client sees the right answer.

What makes it worth a card is that the empty value is indistinguishable from the one case the contract says is real. The operator ruled that an attachment's path is optional, so an absent path means an implementation that cannot supply one. Here it means the code did not bother. Whoever next reads this record and believes it will conclude the attachment has no locatable file, which is false, and the conclusion will look correct because it matches a documented case.

The repair is to fill it in where the writer already has it. The care needed is in deciding whether the field should be absent or filled for every producer of this record, so the answer does not depend on which path through the code created it. A field whose meaning depends on who set it is the defect this workbench has paid for more than once, most recently on dinah-281, where one field carried two unrelated facts and a client could not tell them apart.

Filed out of the first code review of dinah-334, recorded there as a minor rather than something holding that card.
