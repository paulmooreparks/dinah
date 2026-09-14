---
title: The tool refuses a workbench that is too old, and the published profile never says so
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Dinah refuses a workbench definition whose declared profile revision is older than the floor it still supports, as well as one that is newer than the revision it implements. The published core profile describes only the second refusal. Somebody building a second implementation from the document alone would not refuse the old ones, and their tool would pass the conformance suite while behaving differently from the reference on a case a real user reaches after any long gap between upgrades.

This was found during the code review of dinah-123, which retires the rule covering the newer half and publishes a replacement comparing the full version rather than the major alone. The older half was outside that card's scope and is recorded here rather than folded into it.

The gap matters because of what the profile is for. The document is the contract two implementations are held to, and the conformance suite is what keeps Dinah and Andoneer honest against each other rather than shared code. A refusal the reference performs and the contract omits is a place where the suite cannot catch a divergence, which is the failure mode the whole arrangement exists to prevent.

What is wanted is a decision about whether the floor belongs in the shared core at all, and then either a published statement covering it or an explicit entry in the boundary table ruling it out with a reopen condition. Both are legitimate outcomes and the card should not assume the first. Read what the window constants actually do before writing either, because the floor and the ceiling are not symmetric and a statement that describes them as though they were would be a fresh false claim in the document this board has spent a week correcting.
