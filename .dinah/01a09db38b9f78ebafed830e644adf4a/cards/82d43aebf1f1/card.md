---
title: The core profile reads as binding and binds nothing, and three specs have now cited it as settled
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Filed 2026-09-09 after the third instance in two days.

`docs/spec/core-profile.md` is written as a specification: numbered statements, requirement language, the vocabulary of a contract. It sits on the maturity channel `dev`, where the document's own text says nothing below it binds, and the conformance tests confirm the statements it declares are unimplemented. So it describes a thing this project intends and not a thing the tool does.

Three specs have now cited it as though it were in force, each in good faith and each requiring a correction.

Two on dinah-449, where CORE-GATE-1 and CORE-GATE-2 were described first as ratified and then, after a design review corrected that, still needed a second pass to state the channel accurately. One on dinah-436, where CORE-LINK-4 was cited as a settled requirement and explicitly as not the operator's call.

In every case the conclusion drawn was right and only the authority claimed for it was wrong, which is what makes this expensive rather than dangerous: nothing shipped incorrectly, and each instance cost a review round and a rewrite. It will keep costing that, because the failure is not carelessness. An agent that opens the file finds numbered requirements written in requirement language, and the caveat lives in a channel declaration somewhere else in the document. Reading it as binding is the natural reading.

**What this card has to settle, and the first question is whether the document should change or the readers should.**

One answer is that the document should say what it is at every point a reader could enter it, rather than once at the top. A statement that binds nobody could say so beside itself, or the numbering could carry the channel, or the file could open with the sentence that keeps being missed. That is cheap and it fixes the natural reading rather than asking people to read more carefully.

Another is that the tool should say it. `dinah check` already reports things about a workbench; a conformance surface that reported which declared statements are implemented and which are not would let an agent establish the answer by running something rather than by reading prose, which this board's own standing rule prefers.

Another is that the readers should change: a line in the Spec column's instructions telling every spec agent to check the channel before citing that document. That is the cheapest to do and the weakest, because it is another paragraph in a long instruction competing with everything else in it, and this board has watched written rules fail to hold three times this week in other places.

A fourth is that this is not worth fixing, because the document is young and will move off `dev` when the statements are implemented, at which point the misreading becomes correct. That answer is defensible and should be weighed rather than dismissed; the cost of the status quo is measured in review rounds, and there may not be many left to pay.

**What to establish before choosing.** How many statements the document declares, how many are implemented, what the channel actually says and where it says it, and whether any other document on this project carries the same shape. Read rather than assume: the numbers above are what three cards reported and none of them was checking this question directly.
