---
title: the Merge column tells agents to push the trunk, which branch protection forbids
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
The Merge column's instructions describe landing a card as a push straight at the trunk, `git push origin HEAD:<trunk>`. GitHub refuses that on this repository. `main` carries a ruleset requiring a pull request and four passing status checks, so the documented step fails for every card that reaches Merge.

Hit on dinah-213. The merge agent recovered on its own by running `gh pr merge --squash --delete-branch` against the card's already-open and mergeable pull request, which produces the same single squashed commit on the trunk and satisfies the ruleset. That landed as e607816 and the content is byte-identical to the branch head, so the outcome was right. What is wrong is that the column's written contract does not describe the route that actually works, which leaves each Merge pass to rediscover the same refusal and improvise.

Write the pull-request route into the column's instructions as the normal path for a protected trunk: verify the PR is open and mergeable, squash-merge it with the `<human_id>: ` prefixed subject the Commit-prefix directive specifies, delete the branch, then read the resulting hash off the trunk for the COMMIT note. Keep the direct push documented for a board whose trunk is unprotected, and say which condition selects which.

Worth deciding at the same time whether Merge should verify the PR's checks are green before merging rather than relying on GitHub to refuse, since an agent that reads the refusal as a routing problem may go looking for a way around it.
