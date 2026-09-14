---
title: Four small prose findings left behind by dinah-283's quoting rounds
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
dinah-283's sixth code review raised three small prose findings and a nit, and its seventh round was scoped by the operator to two other defects, so all four were deliberately left. They are named in that card's comments rather than lost, and this card exists so they are worked rather than read. A cosmetic leftover from the same rounds belongs here too: the rewrap left one comment line in internal/verb/definition.go noticeably longer than its neighbours, at roughly line 595 as the branch merged.

None of them affects behaviour. That is exactly why they need a card: nothing will fail because of them, so nothing will surface them again.

Read dinah-283's fifth, sixth and seventh review comments for the findings themselves rather than working from this description, because the wording of each finding is the finding.

One thing to carry into the work, since it is the lesson that card paid seven rounds for. Both of the defects that forced its final round were introduced by a round that was tidying prose rather than changing behaviour, and each rewrite quietly widened a promise the code does not keep. So the discipline for this card is the one dinah-283 arrived at: for every sentence, ask what would have to be true for it to be false, and check the answer against the test sitting beside it rather than against the sentence above it. Do not tidy anything you have not falsified.

Scope is prose and one wrapped line. If any finding turns out to need a code change to make its sentence true, stop and say so rather than making it, because that would be a change to a rule the operator ruled on twice.
