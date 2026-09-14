---
title: One rule in the conformance scanner has no test, and it is the one that decides where a doc comment ends
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
The scanner in internal/profile/conformance_test.go decides whether a test names a statement, and part of that decision is working out where a test's doc comment ends. One condition in that decision has no fixture: a blank line inside an open doc-comment header does not close it. The behaviour is correct today and nothing proves it, so a later edit can change it and every test will stay green.

This is the residue of dinah-5, which closed the same class of gap for a sibling rule. That card added a fixture for the rule that a line of code ends a doc comment, after the reviewer showed the rule could be switched off with the whole package still passing and the tree-wide report byte-identical. The blank-line condition was found in the same pass and deliberately left, because closing it was a new fixture rather than a fix to what that card shipped.

Two people have already checked this rather than assumed it. The code reviewer worked out which condition was uncovered and judged that a three-line fixture closes it. The test stage then probed the behaviour directly, confirmed it is correct and genuinely untested, and agreed the reading. So the work here starts from something verified.

What makes it worth doing rather than noting. The scanner is what decides whether a normative statement counts as exercised, so a silent change to where a doc comment ends silently changes what the audit believes about coverage, and the audit is the thing that is supposed to catch exactly that class of drift elsewhere. An untested rule inside the instrument that measures test coverage is the sharpest form of the defect this board has been closing all week.

Write the fixture the way dinah-5's was written: prove it by switching the condition off and watching the new test fail alone, rather than by adding a case that passes. Check while you are there whether any further condition in the same function is in the same position, since two have now been found by looking and neither was found by the tests.
