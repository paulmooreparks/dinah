---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:52Z
ordinal: 28
note: "`dinah attach workstream/addressing f.txt` succeeds at 22a35fc and `dinah attachments workstream/addressing` lists the result, but neither addressing/attachments/1 nor workstream/addressing/attachments/1 resolves, because ResolvePath's workstream arm reads everything after the prefix as the workstream's name. Inventing a spelling here would extend the grammar to a kind dinah-456 section 5.6 rules stays outside the containment table, which is a larger decision than this card holds. dinah-459 refuses the write and reports the files already written, so the row disappears rather than gaining an address. The listing prints what AttachmentView.Ref carries, which after this card is the prefixed form, and the row is honest about naming something the grammar does not reach."
---
An attachment hanging from a workstream keeps its unresolvable reference, and dinah-459 owns it.