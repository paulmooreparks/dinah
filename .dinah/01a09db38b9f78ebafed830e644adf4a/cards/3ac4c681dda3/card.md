---
title: Nothing parses the workflow files, so a workflow can be broken beyond running and the whole suite stays green
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
---
A workflow file was edited during dinah-398 until it was no longer valid YAML, and all 332 tests passed. Nobody noticed, because nothing in this repository parses a workflow file. The tests that touch workflows read them as text and search for strings, which tells you a line is present and tells you nothing about whether the file would run.

That makes every workflow test on this board a check that cannot fail in the way that matters. A test asserting a guard's text appears inside the right step still passes on a file GitHub Actions would reject outright, and the first place the breakage shows up is a release that does not happen.

Two cards landing now lean on exactly this kind of test. dinah-398 asserts the shape of the extension release workflow by slicing its text apart, and dinah-396 extracts a shell check out of the release workflow and runs it. Both are better than nothing and neither would survive the file being malformed.

The fix needs a decision that is not obvious, which is what to parse with. The extension side already has a JavaScript toolchain, the CLI side is Go, and the workflows are consumed by neither. Whichever parser goes in has to be a dependency somebody is willing to keep, and it has to be reachable from whichever test layer the workflow tests live in.

Found by the implementing agent on dinah-398, which caught its own breakage by hand and repaired it before committing, so nothing broken reached a branch.
