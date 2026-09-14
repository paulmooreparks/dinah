---
title: The promotion workflow still names the six platforms by hand, so adding a seventh ships six from the stable channel with no error
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
dinah-396 moved the six platform targets into a single Go declaration and made the release workflow read it, so the dev channel can no longer disagree with the list. The promotion workflow was left alone, and it still spells all six platforms out by hand with no check of its own.

The consequence is quiet. Somebody adds a seventh platform where the declaration says to add it, the dev release carries seven, the promotion carries six, and nothing anywhere reports a problem. The stable channel is simply short a binary and the failure surfaces as a user who cannot find a download.

The fix is to have the promotion read the same declaration the release now reads, or to check it against that declaration. Which of those is right depends on whether the promotion is meant to be able to ship a subset deliberately, and that question should be answered before the code is written rather than assumed.

Found by the code reviewer on dinah-396, which had written into its own Go file that the declaration was the only place the platforms are named. That sentence was false while this workflow stands as it is, and dinah-396 corrects the sentence rather than the workflow.
