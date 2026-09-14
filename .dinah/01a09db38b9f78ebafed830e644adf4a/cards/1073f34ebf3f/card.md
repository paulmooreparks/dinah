---
title: The refusal-name list claims to be complete and nothing checks that it is
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
tier: workhorse
---
Found during the code review of dinah-202 on 2026-08-26, while checking an addition that implementer had made on his own initiative and flagged for scrutiny.

A list in the tree carries a comment claiming to hold every refusal name the product mints, and a guard filters through it. The implementer of dinah-202 added his new refusal name to it because of that claim, rather than because the spec told him to.

**He was right, and the list's claim about itself is currently true.** The reviewer enumerated every refusal name in the tree by command and compared both directions: forty minted and forty listed on the branch, thirty-nine and thirty-nine on trunk. So the invariant holds today and dinah-202 maintained it rather than patching a gap.

**Nothing enforces it.** The reviewer deleted the newly added name from the list and the entire repository suite stayed green. With a name absent, a guard that exists to catch a refusal shipped without a definition loses two of its four assertions, silently, and reports success.

So the list is a completeness claim maintained by whoever happens to remember, and the guard resting on it quietly proves less every time somebody forgets. That is the shape this workbench has spent the week eliminating: a check whose reach depends on a hand-kept list that nothing forces anyone to keep.

## Why this is filed separately rather than pushed back

The defect predates dinah-202 and that card behaved correctly. Holding a card for a weakness it did not introduce and did in fact honour would be the wrong incentive, and the reviewer said so.

## What the fix has to establish

**Where the list's members can be enumerated from.** The reviewer's comparison ran as a command over the tree, so the enumeration exists as a technique already. Whether it becomes a test, or whether the list stops being hand-kept and is derived, is the design question.

Prefer derivation. A test comparing a hand-kept list against an enumeration keeps the list and adds a guard over it, which works. Generating the list from the enumeration removes the thing that goes stale. This board has now landed the declare-rather-than-infer shape three times in one week, on parameter fields, on cross-head identity and on published wrapper members, and the same reasoning applies.

**What the guard actually loses when the list falls behind.** The reviewer reports two of four assertions going quiet. Establish which two and what they were catching, because that says how bad a stale list is rather than merely that it is untidy.

**Whether any other list in the tree carries the same shape of claim.** A comment asserting completeness over a hand-kept collection is a pattern rather than an instance, and this is the second one found this week, after the cross-head identity set that turned out to be derived from doc comments. Enumerate them by command rather than by memory.

## Related

dinah-282 builds the guard that catches a capability reaching one head and not another, and its whole subject is checks that cannot fire. This is the same failure one layer down: a check that fires, over a set that quietly shrinks.
