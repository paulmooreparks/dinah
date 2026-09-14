---
title: A test says it is the only one of its kind, and a later test made that false
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
One test in the tree suite claims in its own comment to be the only place in the suite where a card is actually blocked at a column that takes work up. Another test forty lines below it does the same thing, so the claim is false.

The second test parks two cards in the ready state and then blocks one of them, and the state it parks them in belongs to a column the fixture declares as a work column. That is exactly the situation the first comment says happens nowhere else.

Neither test is wrong and neither needs changing. Only the sentence is wrong, and it should say the test is one of two places where a card stands blocked at a column that takes work up, and that this test's own job is the empty-active-group half of the pair.

The way it became false is the part worth recording. One card wrote the exclusivity claim and a later card falsified it by adding the second test, with nothing in between to notice. A comment asserting that something happens in exactly one place is a claim about the whole suite, and nothing checks a claim like that, so it decays silently the moment somebody writes the second one.

Filed out of the second code review of dinah-322. The reviewer found it while checking five comments that card's implementer had searched out and judged still true, and it is the one of the five that does not survive. It predates dinah-322 on both sides and that diff touches neither the comment nor the test, so correcting it there would have widened a card that was gating the sidebar.
