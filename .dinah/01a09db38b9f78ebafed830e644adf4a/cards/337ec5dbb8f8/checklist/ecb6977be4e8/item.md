---
kind: decision
state: resolved
ts: 2026-09-14T02:18:04Z
ordinal: 22
note: The guide says today "If the collection you name holds nothing, Dinah tells you that nothing answers to the reference rather than telling you the collection is empty", which is true at b825059 and is exactly the behaviour dinah-456 section 3.3 rules out. The rewrite says a collection holding nothing is an empty answer rather than a mistake. That literal is in the form list of TestTheReferencesGuideSaysWhichCommandTakesWhat, which opens at main_test.go:7172 and reports through the Errorf at :7181, and it has to come out with the sentence, which the observed test run confirmed.
---
The empty-collection sentence changes from what the tool does today to what the contract says, and the literal that pinned the old sentence comes out of main_test.go.