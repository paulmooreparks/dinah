---
title: Nothing holds the format document's event accounting to the code that produces it
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The format document states, in prose, how many event kinds this build declares, how many it writes, and how many are queryable over a card. Those numbers describe the code, and nothing checks them against it. They have now been wrong twice within a single card.

On dinah-314 the accounting sentence shipped wrong in three separate ways at once. It placed an event in the written set that the sentence six lines above it correctly called unwritten, it claimed two events were written by nothing when a fixture on disk proved otherwise, and its total was off by one. A code review caught all three by counting the constants by hand. The repair round then rewrote the sentence and strengthened the acceptance criterion so that it derives the three sets from the code rather than merely asserting a sentence exists.

That criterion is the ratchet, and it only turns when a person runs it. Between one card and the next the document is free to drift from the code again, and the next drift will be found the same way this one was, which is to say by somebody being suspicious enough to count. A card whose whole subject was a document promising something nothing backed should not leave its own accounting resting on that.

What is wanted is a check that runs in CI and fails the build when the document's numbers or memberships disagree with the constants and the query vocabulary. The derivation already exists in the strengthened criterion on dinah-314, so the work is to move it from something a reviewer runs into something the build runs, and to decide where such a guard lives given that the repository already has fixtures keyed to source lines and those have proven brittle.

This was cut from dinah-314 deliberately, on the grounds that it is new machinery rather than a repair of that diff. The implementer declared the gap rather than leaving it to be discovered.
