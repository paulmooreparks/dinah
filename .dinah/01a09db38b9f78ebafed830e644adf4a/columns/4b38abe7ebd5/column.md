---
title: Agent Code Review
slug: agent-code-review
kind: work
reject_to: implement
loop_limit: 3
operator_owned: false
gate_items: out
---
A fresh agent reads the implementer's diff against the card's contract. Clean, and the card goes forward to Operator Code Review; findings, and it goes back to Implement, which this column declares as its reject target. The reviewer works one tier above the card's own.

**Every diff on this workbench is read here, on every card without exception.** That is carried by this instruction and by the fact that every card walks the whole route, rather than by any refusal in the tool: nothing stops a move that steps over a station. It matters more than it used to, because the operator ruled on 2026-09-14 that his own stations let a clean card through, so between Implement and the trunk this station is the reading that always happens.

What to check: correctness against the card's contract and its edge cases; secrets handling and authorisation on every entry point that has one; the codebase's own idioms; whether the implementer ran what this workbench asks of Implement; and whether the cuts are honest.

## Begin at the code

Reading the diff is where this review starts and where most of it happens. Implement already smoked the change and Test owns the authoritative execution pass, so a clean read is a complete review. Do not build, run a suite, or exercise a fixture as a matter of course. A routine build on a diff that read clean buys nothing Test will not buy properly an hour later, and it is how this column loses the cheapness that lets it run on every card.

Expand into execution when, and only when, reading has surfaced a specific problem you must understand before you can write the response. That is the trigger: a concrete question the diff raises and the source will not answer, not a wish for more confidence. Once you are there, run what the question needs, and the scope follows the question rather than a fixed permission.

When you expand, record it in the findings comment: what reading could not settle, what you ran, and what it showed. That is the difference between a justified expansion and drifting back into being a second Test.

When you suspect a defect in behaviour and choose not to chase it, either push back with the concrete suspicion, or pass and name the check in your findings comment as a directed target for Test. Named targets are contract that Test picks up.

## Trace a claim rather than reading the function it names

A claim that something happens in one central place, so every caller behaves alike, is checked by following each caller's path and not by reading the callee. Reading the function supports the claim every time. One card here spent three review rounds on exactly that, and tracing found three of six callers wrong, each differently, the worst being a caller that never failed at all and printed a live answer. Say which paths you followed.

## A change without a test is a blocker, not a nit

Implement is required to deliver a test for the behaviour that changed, and this column is where that is enforced. Push back when the diff changes behaviour and nothing in it exercises that behaviour. An extended existing test satisfies the rule and is the preferred shape; a new file covering ground an existing test already owns does not.

Check also that the implementer armed the test: broke the behaviour, watched it go red, restored, watched it pass. A test nobody has ever seen fail is not yet evidence that it guards anything.

Four failure shapes are worth knowing, because each passes a careless read. An assertion matching a substring that the old behaviour also produced is not a test of the change. An expected value computed by calling the thing under test moves with the defect and can never fail. A sweep whose subject set can be empty passes by having read nothing, so a sweep asserts how big its set was. And a run piped into another command reports the pipe's exit status rather than its own, so a failing run sent through a pager reports success.

## A guard widened by example fits only its examples

When a check matches text and somebody defeats it, the reflex is to widen the pattern by the spelling that escaped. Do not. A guard on this project was widened that way twice and broken twice, and the fix was to stop matching text and parse the construct instead, after which fourteen spellings by three reviewers failed to escape it. When you find yourself asking for a third variant of one regular expression, ask instead for the construct to be parsed, or for the guard to be deleted and the gap stated plainly in the code. A guard reporting success while the thing it watches for walks past is worse than an honest gap.

Where a guard genuinely cannot see something, the right outcome is that the code says so where somebody meets it, with the reproduction that demonstrates it. Narrowing a claim to what actually ships is a repair rather than a retreat.

## Check the diff against the house idiom corpus

The workbench attachment named "Convention counterexamples" carries the wrong-versus-right idiom pairs this project has already paid for, and consulting it is part of the review. It is large, so do not read it end to end: match its entries against the surfaces this diff actually touches, and read those.

Report the result: which entries you checked, and which areas you skipped because the diff does not touch them. A review naming no entries has not done this step, and a claim that none applied needs the areas listed to be credible.

When you catch a defect whose class is not yet in the corpus, and the same class has now been caught twice, add it as a new pair. That practice is the only mechanism this workbench has for turning a lesson into something the next reviewer inherits.

## Prose in the diff is reviewed against the prose standard

The workbench attachment named "Prose standard" governs any prose this diff touches. Hold it against that standard's list of tells and record violations as findings.

Its hard constraint cuts both ways here. A diff rewriting existing prose is checked for drift in meaning word by word: a dropped qualifier, a deleted negative clause, or a weakened commitment is a major finding even when the style improved. And a finding you write asking for a prose fix must itself say what to remove without removing what the sentence promises. Name the tell when you record one.

Two shapes this workbench has paid for repeatedly. A paragraph closed with a summarising sentence nobody asked for is how three consecutive rounds on one card each shipped a fresh false claim while repairing the last one; when you catch that shape, ask for the sentence to be deleted rather than checked. And a correction narrowing a premise has to be read against the conclusion drawn from it, because the sentence that was true only under the wider premise is usually left standing right underneath.

## Go style and reuse

The workbench attachment named "Go style standard" is the rule set for Go source in the diff. Take the mechanical floor first, since `gofmt -l .` settles that part without judgement, then read for the judged rules.

Weight findings the way that document weights them. A file gofmt would reformat is a major finding, as is a new function duplicating one the codebase already has. The judged rules are minor findings, recorded but not on their own a reason to send the card back.

Check the reuse claim and not only the code. The handoff either names a helper that was reused or asserts none existed, and that assertion is checkable in one search. A claim absent altogether is itself a finding.

Cite the rule you are applying. A style finding the document does not state is out of scope here, and the way to raise one is to propose it as an addition to the document.

## Copy references are load-bearing

For every user-facing string naming a destination, check that the destination exists or is created in this diff. A plausible reference to nothing is a blocker: the copy gets corrected or the destination gets a follow-on card.

## Where the diff is

Read the branch name from the card body, under the `## Branch` heading, because a Dinah card has no field for it. Then fetch and read the branch against the trunk:

```
git -C <tree> fetch origin
git -C <tree> diff origin/main...origin/<branch>
git -C <tree> log --oneline origin/main..origin/<branch>
```

Three dots on the diff, not two: that is the diff since the merge base, so it stays correct after the implementer merges the trunk into the branch, which he is told to do. The log gives you the commit series, which is how you see what a second round added without re-reading what you already passed.

This code is not in the trunk. Pushing back costs the trunk nothing, which is the whole reason this stage sits where it does.

A recorded branch that no longer exists on the remote means the card was already merged, because Merge deletes both copies when it lands the squash commit. That is the ordinary end state of a merged card, so do not read it as a missing branch. Read the squash commit on the trunk instead. If the trunk carries nothing for the card either, the work is genuinely nowhere, and that is a push-back with the absence stated.

A card whose body carries no `## Branch` heading and whose deliverable was code is a push-back on its own: the implementer skipped recording it and nobody downstream can find the work.

## A card whose deliverable is not code

Some cards deliver a document, a transcript or a change to this workbench's own configuration rather than a commit. Such a card has no branch and no diff, so the commands above find nothing, and that absence is the expected state rather than a push-back. Tell the two apart by the deliverable the card names, never by the absence alone.

Review the artifact itself. Read the live surface with `dinah show`, `dinah instructions <column>` or `dinah attachments` as the deliverable demands, and hold it against the card's contract, against the prose standard, and against the reference rule above. The findings format and the loop limit are unchanged.

### This column holds on the way out

Your own items are settled here, so file them with `--column agent-code-review` and settle them before the card leaves. The refusal is `dinah.unresolved-item-exit`, and it refuses a push-back as readily as an advance: to send a card back to Implement you settle your items first, or move one with `dinah set <item> column <other column>`.

### The loop limit

This column declares a limit on how many times it may send one card back. Read the card's standing against it from `dinah show <card>` rather than counting for yourself. The count is the number of times this column sent the work back, not the number of times a card arrived here, and those differ: a card pushed back once by this column, then failed by Test and returned through Implement, arrives here three times having been rejected here once. A release or a block never counts, because neither is a move.

At the limit, post an escalation comment with the current round's findings, block the card, and do not move it. The operator decides what happens next.

One judgement the gate does not encode: escalation exists to put a ruling in front of him while findings are still open. Reaching the ceiling on a clean read, with nothing blocking and nothing major, is a card that finished rather than a card that is circling, and blocking it asks him a question with no content in it. Say plainly which of the two you are looking at.

### Findings, and the handoff

Post findings with `dinah comment <card> "<text>"` under a `## Findings` heading, tagged blocker, major, minor or nit, and carry the counts in the move note. Cite a checklist item by its Dinah reference, in the form `dinah-123/criteria/3`. Read the implementer's handoff and the prior comment thread first, so you do not repeat a finding somebody already made.

The move note carries the pull request's link, any pending questions stamped for the operator, and the one-line acceptance criterion an artifact's approval would create, or the plain statement that nothing awaits him. A clean card passes his station without stopping, so anything you do not name there is anything he will not see.
