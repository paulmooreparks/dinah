---
title: Nothing requires the operator to read a diff before it lands
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
Noticed on 2026-09-08 while merging dinah-435, and worth a decision rather than a shrug.

The repository ruleset requires a pull request with its status checks green, and it requires zero approving reviews. So a card can reach the trunk with no human having read its diff. That is not a defect in any card or any stage; it is what the configuration currently says, and the Merge column's own instructions already describe it accurately.

What makes it worth deciding now is that two other things changed this week and the three together close the last gap.

The `dinah-gh` identity exists precisely so that the operator keeps his Approve button, because GitHub will not let the author of a pull request approve it. Every pull request on this board is opened under that identity for that reason alone. But nothing consumes the button: the ruleset does not ask for an approval, so the machinery preserves an option nobody is required to exercise.

The fast-track authority of 2026-09-04, widened on 2026-09-08, says a card carrying no ruling for the operator travels through his stations rather than stopping at them. That is the right call for a clean card and it is working. Its consequence is that Operator Code Review, which is where he reads the pull request, is skipped exactly when the card looks fine, which is most of the time.

Both cards that landed on the trunk on 2026-09-08 took that route. dinah-206 minted a published surface, six commands, six refusal names and six journal events that any other implementation of the contract will have to honour, and it merged without the operator reading its diff. dinah-435 changed printed output he had ruled on twice, and merged the same way. Both had passed agent code review, both had a full Test stage, and neither was rushed. That is the point: the route is sound and it still ends with nobody having looked.

What this card has to settle is which of three things the operator wants, and it is his call rather than a technical question.

Require an approving review in the ruleset, which makes the Approve button load-bearing and puts him back in front of every diff. Cheap to configure, and it turns every merge into a wait on him, which is the cost the fast-track authority was granted to avoid.

Leave it as it is and say so deliberately, on the grounds that two agent reviews and a Test stage are the gate, and that his stations exist for rulings rather than for reading code. That is a defensible position and it is currently the de facto one; making it explicit is worth something on its own.

Require an approval only for cards that touch a published surface, which is where an unread diff actually costs something later. A contract, a refusal name, a journal event and a command are hard to withdraw once another implementation depends on them, while a render change is not. This is the narrowest option and the hardest to configure, since GitHub's rulesets do not know what a published surface is.

Whoever works this card should establish what the ruleset actually says today by reading it rather than by repeating this description, since the description rests on one merge agent's report and on the Merge column's prose.
