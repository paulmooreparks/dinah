---
title: The release workflow's collision and cleanup paths have never run, in a test or for real
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
workstreams:
  - 58f3e3eb621a
---
The extension release workflow works. It ran three times on 2026-09-06 and produced 1.0.0, 1.0.1 and 1.0.2, minting each version from the release history and reserving the tag before building. Every one of those runs took the happy path and succeeded.

Two paths inside it have still never executed, anywhere. The first is the refusal that fires when two merges race and the second run finds the tag already reserved. The second is the cleanup job that deletes a tag when a run fails after reserving it. Neither has happened for real, because nothing has raced and nothing has failed, and no test drives them, because the tests here read the workflow file rather than running it.

The race in particular cannot be settled by reading. It turns on what the API returns to the second caller at the moment the first has already reserved the ref, and on the workflow reacting to that answer correctly. A document review establishes that a step exists and what it says. It cannot establish what the step does when the answer comes back.

This card was originally titled as though no release workflow had ever run, which was wrong and was corrected on 2026-09-06 after Paul caught it. The three successful releases are the evidence that the ordinary path is sound. What remains unproven is narrower and it is the part nobody sees until the day it matters.

What this card owns is verification against a real run rather than against a file, and the first thing it has to settle is where such a run happens. Forcing a collision or a mid-run failure on the real repository creates real tags and real releases, and Paul has been clear that a permanent public release is his call. A scratch repository, a fork, or a dry-run mode are all plausible and they are not equivalent.

Related: dinah-401 covers the fact that nothing parses the workflow files, which is a different gap and largely answered for the extension workflow by dinah-399 adopting a parser. Reading a workflow correctly still says nothing about what it does when it runs.
