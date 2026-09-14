---
title: A test comparing the thing under test against itself
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
One assertion checks that the conformance claim the tool reports differs from the constant the tool reports it from, and the function under test returns that constant verbatim. So half of that test compares a value against itself and can never fail, whatever the code does. It was introduced while rewriting tests for the compatibility work and found by the test pass rather than by either review, because it reads as a reasonable assertion and only becomes empty once you follow what the function actually returns. Its two sibling assertions in another file are a different shape and are correct.
