---
title: The interchange example shows a version field without saying whose version it is
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The interchange section of the core profile shows an example workbench declaring a profile revision, and nothing nearby tells the reader that the number is the document's own revision rather than the version of any tool. A reader who copies that example into a real workbench gets a refusal from a build whose conformance claim has not caught up, and the document gives them nothing at that spot to explain why.

Nothing here is broken and the arrangement is deliberate. The operator ruled that a build may declare conformance to an older revision than the document publishes, and a guard now requires every live statement of the revision in that document to agree, so the example cannot lag without reddening the trunk. The document also says, in its own words elsewhere, that the profile's version is unrelated to the release numbering of any tool. That sentence is the one a confused reader needs, and it is nowhere near the example that confuses them.

The same property has been true of the conformance-claim example a few hundred lines earlier for as long as that example has existed, and nobody has ever called it a defect, so this is a gap in the document's helpfulness rather than a correctness failure. It surfaced during the code review of dinah-123, which took the document to a revision the current build refuses, making the example copyable and wrong for the first time in a way somebody noticed.

What is wanted is a sentence in the interchange section saying what the version field carries and what it does not, placed where a reader meets the example rather than a few hundred lines away. Check the conformance-claim example for the same need at the same time. Write the sentence to survive the next bump, since it will sit inside a document whose revision moves, and say nothing about which revision any particular build accepts, because that is exactly the coupling the operator ruled against.
