---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:00Z
ordinal: 18
note: "Both spellings resolve today and both keep resolving; this is about what gets printed. Verified at 22a35fc that show prints pb-1/oq/1 and contents prints pb-1/checklist/1 for the same item on the same card. The aliased form carries the item's kind in the address, so a reader who copies it out of a listing can still tell an open question from a criterion, and it is the form dinah-435 established. show has many readers where the containment walk has one, so the walk is the surface that moves.\n\nAMENDED, round 3: the alias is the long word rather than the short one. The operator ruled on 2026-09-09 at dinah-454's Operator Code Review that `oq` becomes `questions`, `ac` becomes `criteria` and `d` becomes `decisions`, with the three short forms still resolving on input and documented as accepted rather than deprecated. His words: \"The term 'oq' is handy as text-speak, but it's a bit non-obvious, especially if we use 'attachments' and 'comments'. It follows that we should use 'questions' instead of 'oq'.\" Nothing about which of the two spellings is printed changed; only which word the aliased spelling uses. Spec sections 3.1 and 4.3 carry it, and dinah-454 lands it, because that card's own address guard is what proves the rename broke nothing. The kind tokens open_question, acceptance_criterion and decision are untouched, which spec section 11 records as out of scope."
---
A checklist item is printed as the aliased form, and the alias is the long word `<card>/questions/<n>`, with `contents` changing to match `show` rather than the reverse.