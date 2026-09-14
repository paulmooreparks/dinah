---
title: The test guarding the publish script's error message passes whether or not the fix it guards is present
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
workstreams:
  - 58f3e3eb621a
---
The publish script looks up the newest release to decide what version to publish. When no release exists, the helper it calls writes its explanation to the error stream, and the script captures that stream so the message reaches the script's own reporting instead of a blank failure. That capture was written during dinah-399 after two rounds of design review caught it missing, and the message on an empty release history is the first thing Paul will actually see, because the repository has no extension release tags at all.

The tests that guard it search the process's standard output and error glued together. PowerShell puts the message on the error stream whether the capture is there or not, so removing the fix leaves every assertion passing. The code reviewer on dinah-399 ran it both ways rather than reasoning about it and confirmed the guard is blind.

The code is right. The guard around it is not. A test here has to establish which stream carried the message, not merely that the message appeared somewhere in the combined text.

This is the same shape the board has closed repeatedly: a check that cannot fail. It is worth noting that this one survived three rounds of design review and two of code review on the card that introduced it, because the assertion looked specific while the thing it asserted was true either way.

Found by the code reviewer on dinah-399, recorded rather than sent back, since the shipped behaviour is correct.
