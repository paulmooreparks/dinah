---
title: the coverage guard gives up and says nothing whenever another test is red
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
`TestEveryStatementOfTheRenderingHeadIsCoveredOrNamed` in `cmd/dinah` shells out to `go test -coverprofile` over its own package and reads the profile back. When that child run fails for any reason at all, the test reports "the coverage run failed, so nothing below can be read" and stops. So any single red test anywhere in the package blinds the coverage guard completely, and the guard's own result is unavailable exactly when the package is in the state where somebody most wants to know it.

It has a second blind spot in the other direction: it skips itself under any test filter, on the reasonable grounds that a filtered profile would report the filter rather than the suite. Together those mean a filtered run says nothing and an unfiltered run with one unrelated failure also says nothing, and both look like a pass to a careless reader.

This has already cost real time. On dinah-213 a genuine coverage failure of mine sat behind the cascade for two full review cycles: the local suite was red for an unrelated environment reason, the guard reported only its opaque message, and CI was the first thing able to see the actual four-line finding. Two reviewers and I all read the local run as "two known failures" and moved on.

Raised while spec'ing dinah-229, which fixes the environment leak that was cascading into it. That card deliberately does not fix this, and the reasoning is worth keeping: a coverage profile does get written even when tests fail, so the guard could read the partial profile instead of giving up, but a profile from a run that died early under-reports coverage and would invent findings against blocks that simply never executed. Choosing between "report nothing" and "report findings that may be false" is a contract worth writing down rather than improvising inside a test.

Note there is no hidden backlog behind this. Once dinah-229 closes the environment leak the guard runs and passes as it stands, so this card is about the guard's behaviour under future failure rather than about anything currently broken. Sequence it after dinah-229.

The spec should settle what the guard does when its child run fails: whether it reads the partial profile and reports only blocks it can prove were reachable, whether it distinguishes a compile failure from a test failure, whether it says which test failed rather than that one did, and what it does under a filter. Whatever is chosen, the failure mode that matters is the one this card was filed for, which is the guard being silently uninformative rather than loudly wrong.
