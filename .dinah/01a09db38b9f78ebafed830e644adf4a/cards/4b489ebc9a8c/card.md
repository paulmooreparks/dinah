---
title: Three contract rules are credited to a test that proves the opposite of what they require
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
Found on 2026-09-10 by a mechanical sweep during the code review of dinah-436, and it is on the trunk rather than on that card.

The conformance suite decides which contract statements Dinah's behaviour is exercised against by reading a claim in each test's own comment. On that path the comment is the record rather than a description of one, so a comment that overclaims is a false entry in the coverage register rather than a documentation slip.

An ordering test in `internal/verb/mutate_test.go` is credited with covering three rules that each require the refusal named `held`. No case in that test reports `held`, and none can: the test exists to prove that some other refusal fires first, so the state where `held` would be returned is the state it is written to avoid reaching. The test proves the opposite of what it is credited with.

Nothing else in the tree covers those three rules. So the register says three requirements are exercised, and no assertion anywhere would go red if the tool stopped honouring any of them.

**How it was found, which is the part worth keeping.** Two review passes on dinah-436 had already caught three comments of this shape by reading them against their tests, and both passes missed this one, because reading is what lets the class through. The sweep that found it discarded the comments entirely: for every rule demanding a specific refusal name, take the tests credited with covering it and require the code to assert that name. That check is now in the conventions corpus so the next reviewer runs it rather than reads.

**What this card has to settle.** Whether the three rules are honoured by the tool at all, which is a separate question from whether anything checks that they are, and should be answered first by exercising the behaviour rather than by reading the code. Then either write the assertions that genuinely cover them, or, if the rules turn out not to be honoured, say so plainly, because a rule the tool does not honour and a rule nothing checks look identical from the register and are entirely different problems.

**A second thing to settle, larger than the three rules.** Crediting coverage by a sentence in a comment is what made this possible, and the same mechanism guards 94 statements across 64 tests. The sweep run during this review screened all 64 and hand-read 33, finding only this one, so the register is mostly honest today. It is honest by the diligence of whoever wrote each comment rather than by construction, and nothing stops the next comment from being wrong. Decide whether the crediting mechanism should demand something the code carries rather than something its author typed, and weigh that against the cost, since a stricter mechanism that nobody can satisfy gets exemptions added until it means nothing.

Related to the open card about that profile document being read as binding when it is not; this is the same document read from the other side, where the question is not what the statements oblige but whether anything demonstrates the tool meets them.
