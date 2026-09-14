---
title: A pull request merges only when it is up to date with the trunk
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
A branch can pass every check against one trunk and then merge into a different one, because the repository does not require a pull request to be current before it lands. That gap stopped being theoretical: two cards each passed review and test against different trunks, collided in the same test file at merge, and the resolution cost an integration pass and came close to dropping a closing brace. Turning the requirement on costs a rebase whenever a branch falls behind during review, which is minutes given how fast the checks run. The setting lives in the repository's branch ruleset rather than in the code, so this is a decision about how the project merges rather than a change to the tool.
