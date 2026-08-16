---
title: Code Review
kind: work
operator_owned: false
---
A fresh agent reads the implementer's diff against the spec: clean, forward to the lane's next stage; findings, push back to Implement/Ready. The reviewer runs one tier above the card's expected_tier.

**Take the forward destination from the lane, not from this sentence.** Read the lane block on your claim response for the card in front of you; it wins over any column that names a neighbour by hand.

Checks: correctness against the spec and its edge cases; security (secrets handling, authorization on every entry point that has one); style (codebase idioms); self-test level appropriate per the Implement checklist; WHAT I CUT honesty.

## Begin at the code

Reading the diff is where this review starts and where most of it happens. Implement already smoked the change and Test owns the authoritative execution pass, so a clean read is a complete review. Do not build, run a suite, or exercise a fixture as a matter of course. A routine build on a diff that read clean buys nothing Test will not buy properly an hour later, and it is how this column loses the cheapness that lets it run on every card.

Expand into execution when, and only when, reading has surfaced a specific problem you must understand before you can write the response. That is the trigger: a concrete question the diff raises and the source will not answer, not a wish for more confidence. Once you are there, run what the question needs; the scope follows the question rather than a fixed permission.

When you expand, record it in the findings comment: what reading could not settle, what you ran, and what it showed. That is the difference between a justified expansion and drifting back into being a second Test.

When you suspect a behavioral defect and choose not to chase it, either push back to Implement with the concrete suspicion, or pass and name the check in your findings comment as a directed verification target for Test ("Test must specifically exercise X"). Named targets are contract that Test picks up. On a lane with no Test stage there is nobody downstream to pick them up, so a suspicion you cannot confirm becomes a push-back rather than a note.

## A change without a test is a blocker, not a nit

The Implement column requires that delivering an implementation means delivering a test for the behaviour that changed, and this column is where that is enforced. Push back when the diff changes behaviour and nothing in it exercises that behaviour. An extended existing test satisfies the rule and is the preferred shape; a new file covering ground an existing test already owns does not, because a suite that grows by accretion becomes slow, then redundant, then skipped.

Check also that the implementer armed the test: broke the fix, watched it go red, restored, watched it pass. A test nobody has ever seen fail is not yet evidence that it guards anything. Two failure shapes are worth knowing because both pass a careless read. An assertion that matches a substring the old and the new behaviour both produce is not a test of the change. And an expected value computed by calling the thing under test moves with it, so the assertion holds in both worlds; the expectation has to be pinned independently.

## Check the diff against the house idiom corpus

The workbench document **"Convention counterexamples"** carries the wrong-versus-right idiom pairs this board has already paid for, and consulting it is part of the review rather than optional enrichment. It is large, so do not read it end to end: match its entries against the surfaces this diff actually touches, and read those.

Report the result in your findings comment: which entries you checked, and which areas you skipped because the diff does not touch them. A review that names no entries has not done this step, and "none applied" is a claim that needs the areas listed to be credible.

When you catch a defect whose class is not yet in the corpus, and the same class has now been caught twice, append it as a new wrong/right pair. That promotion practice is what keeps the corpus worth reading, and it is the only mechanism this board has for turning a lesson into something the next reviewer inherits.

## Copy references are load-bearing

For every user-facing string that names a destination (a URL, a page, a button, a route, a command, a "see X", a "configured under X"), grep that the destination exists or is created in this diff. A plausible reference to vapor is a [blocker]; the copy gets corrected or the destination gets a follow-on card.

## A card whose deliverable is not repo code

The Design lane routes documents, UX sketches and specifications through this column, and some of those land as live content (a workbench document, a project document, a column's instructions) rather than as a commit. Such a card has no branch and no diff, so the git commands below find nothing. **That absence is the expected state for these cards, not a push-back.** Tell the two apart by the deliverable the spec names, never by the emptiness of `branch_name` alone: a card whose spec promises code and produced none is still a push-back with the absence stated.

Review the artifact itself. Read the live surface with `get_workbench_document`, `get_project_document` or `get_column` as the deliverable demands, and hold it against the spec, against the board's writing conventions, and against the reference rule above, which applies to prose exactly as it applies to copy in code. Check the revision or version the implementer named in WHAT SHIPPED against what you read: when they disagree, something moved after the handoff, and you are reviewing a surface the implementer did not hand you. Say so rather than reviewing it silently. The findings format, the tagging and the loop limit are unchanged.

## Where the diff is

Findings in an add_comment under `## Findings`, tagged [blocker]/[major]/[minor]/[nit]; the move-note carries the counts. Read WHAT SHIPPED (for the diff base) and the prior comment thread first; don't duplicate findings.

The card's branch name is on the card, in `branch_name`. Fetch first, then read the branch against the trunk:

```
git fetch origin
git diff origin/main...origin/<branch_name>
git log --oneline origin/main..origin/<branch_name>
```

Three dots, not two: that is the diff since the merge base, so it stays correct after the implementer merges the trunk into the branch, which they are told to do. The log gives you the commit series, which is how you see what a second or third round added without re-reading what you already passed.

This code is NOT in the trunk. Pushing back costs the trunk nothing, which is the whole reason this stage sits where it does.

**A recorded branch that no longer exists on `origin` means the card was already merged.** Merge deletes both copies when it lands the squash commit, and the remote deletion drops the remote-tracking ref at once, so the commands above fail with `fatal: ambiguous argument 'origin/main...origin/<branch_name>': unknown revision or path not in the working tree`. That is the ordinary end state of a merged card, so do not read it as a missing branch and do not push back for it. Read the squash commit on the trunk instead: `git fetch --prune origin`, then `git log --oneline --grep=<human-id> origin/main`, then `git show --stat <hash>`. If the trunk carries nothing under the card's human ID either, the work is genuinely nowhere, and that is a push-back with the absence stated.

A card whose `branch_name` is empty was implemented under a trunk-pushing policy, so its code is already on `origin/main`; diff against the implementer's base SHA from WHAT SHIPPED. A card that should have a branch and does not is a push-back on its own: the implementer skipped recording it and nobody downstream can find the work.

## The loop limit, and what its number counts

Limit 3. **Read the count from the `loop` block served on your claim response rather than computing one.** The block carries `column_id`, `count`, `limit`, `at_limit`, `at_limit_reached` and `basis`, and it is omitted entirely when a column declares no limit, so an absent block means there is no gate here rather than that something went wrong.

**`count` is the number of times this column sent the work back**, not the number of times a card arrived. Those differ: a card pushed back once by this column, then failed by Test and returned through Implement, arrives here three times having been rejected here once. A release or a block never counts, because neither is a move; an operator's regressive move in a browser counts exactly like an agent's; and a move skipping several stages backwards counts once.

At the limit: post `## Escalation` with the current-cycle findings, block_card with reason code-review-loop-limit, don't move. The operator decides next.

Judgement the gate does not encode: escalation exists to put a ruling in front of the operator while findings are still open. Reaching the ceiling on a clean read, with no blocker and no major, is a card that finished rather than a card that is circling, and blocking it asks the operator a question with no content. Say plainly in your verdict which of the two you are looking at, and record the reasoning either way so it can be overruled.
