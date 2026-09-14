---
title: The scratch area has 925 directories and 335 of them are live worktrees
column: 5ea2db0272fc
state: ready
severity: minor
priority: soon
---
Found on 2026-09-08 while diagnosing a cleanup failure that turned out not to be one.

Five agents in one evening reported that removing their worktree failed with a permission error, and one of them described it as a known lingering-handle problem on this board. None of that was true. In every case the removal had succeeded: git no longer listed the worktree and its files were gone. What remained was an empty parent directory, and Windows refused to delete it because the agent's own shell was standing inside it. Deleting the same directory from a shell standing elsewhere worked first time, with no error and no wait.

The accumulated cost is visible: `C:\dinah-scratch\` holds 925 directories while git knows about 335 worktrees. So roughly two thirds of what is down there is empty husks left by agents that believed cleanup had failed and stopped trying, plus older trees from cards long since finished.

Two things this card should settle, and they are separable.

The instruction side. Agents should be told to run the removal from outside the worktree they are removing, and told that a permission error on the final directory is not evidence the removal failed. Both belong in whatever text agents actually read, which is the column instructions rather than a document nobody opens.

The cleanup side, which is the part that needs the operator. Sweeping the scratch area means deleting directories in bulk, some of which may hold work somebody wanted and none of which anybody has inventoried. A safe sweep would delete only what is provably both unregistered with git and empty, which is a small and reversible cut; a wider one would take unregistered directories whatever they contain, which is faster and could destroy something. That choice is the operator's, and until he makes it nothing under that root gets deleted in bulk.

Related to the standing note that worktree removal is refused only from the main checkout, which is a separate false claim about the same operation that this board has already had to correct once.
