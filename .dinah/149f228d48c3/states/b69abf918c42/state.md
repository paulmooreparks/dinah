---
title: Acceptance
kind: work
operator_owned: true
---
Cards whose commit is on the trunk and have not yet earned the right to be called finished.

Landing a change and observing it are different things, and this column is the gap between them. A card arrives here because Merge put its commit on the trunk. It leaves when the change has been exercised there without incident, which is a judgement about evidence rather than a wait for a timer.

Claim the card while you are reviewing it. The rule against working an unclaimed card covers operators too.

A card arriving here with a pending `owner="operator"` open question was misrouted: questions are answered at Operator Review, or at the block where they arose, and never presented for acceptance. If you are the operator, answer the question before judging the card; anyone else blocks the card with the question as the reason rather than working around it.

### What acceptance means here

Something has to have used the change. A command has been run, a changed path has carried real work rather than a fixture. A card whose change nothing has touched has not been accepted; it has merely aged, and moving it on records a confidence nobody earned.

This column and a card's acceptance criteria are different things despite the shared word, and the difference is the point of the stage. The criteria were verified at Test, against the card's own branch with the trunk merged in. This column asks a narrower question: does the change still hold on the integrated trunk, in real use with real data. Where the two disagree, real use wins and the card goes back to Implement/Ready with what differed.

Evidence gathered incidentally counts, and is usually the best kind. A card that ships a command is accepted when somebody's real work ran that command and got the right answer, which beats a run made solely to satisfy this column. Record what was run and what came back, in a comment, so the judgement is auditable rather than asserted.

Any acceptance criterion the card is still carrying is settled here, including one minted by an Operator Review approval earlier in its life. A criterion nobody can close is closed honestly or left pending with the reason recorded. A criterion marked verified on partial evidence is worse than one left open, because nothing downstream will look at it again.

### When something goes wrong here

A defect found in this column is not a new card by default. If the change that caused it is still here, it belongs to the card that shipped it, so push that card back to Implement/Ready and say what happened in real use. Filing a fresh card instead breaks the link between a change and its consequence, which is the only reason this stage is worth having.

A problem that turns out to predate the merge is a separate card, as it would be anywhere else.

### Age here

Age in this column is decision latency and it measures the operator rather than any supplier. It is also the buffer in front of the board's constraint, so keep it short by batching: try several cards in one sitting rather than one at a time.

### Leaving

Move to Done when the change has been exercised on the trunk and held. On this board that is the last gate, so Done means merged and accepted.

**Use `move_card` directly, and do not try to claim a downstream stage first.** This column is a pull queue and Done is an output queue, so no flow column exists downstream: `pull` refuses with `pull_queue_no_downstream` and `claim_card` refuses because it requires a flow column. `move_card` works from here with no such claim, and it is the only exit today. Treat that as a workaround rather than the design: the platform tracks a fix for the pull path dead-ending on a board's last stage. Until that lands, use `move_card` and do not spend time diagnosing the refusal.
