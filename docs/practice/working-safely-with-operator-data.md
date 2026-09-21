# Working safely against the operator's own data

Dinah is developed by a person who also uses it. His workbenches, his settings, and his repositories sit on the same machine every agent on this board runs on, and reaching them takes no privilege and no mistake anybody would call careless. Read this before running the binary for any reason: reproducing a defect, verifying a criterion, or exercising a command to see what it prints.

Every rule below was written after it went wrong.

## The two paths that reach his real data

**Configuration writes to the user base.** `dinah config set` writes `~/.dinah/config.md`, which holds his own settings rather than anything belonging to a card. Set `DINAH_HOME` to a scratch directory before any config exercise, and never run a config write against an inherited environment. An agent reproducing config behaviour changed his language setting to French, then removed the key entirely, while he was watching.

**Discovery walks the ancestor chain to the drive root, unless a repository bounds it first.** A workbench is found by climbing from the current directory upward, and `DINAH_HOME` does not stop this climb, because it relocates the user base rather than bounding the walk. A scratch tree that sits inside a git repository, this project's own checkout worked from a linked worktree among them, is bounded at that repository's root: the climb never crosses out of the repository to reach a workbench above it, wherever inside the repository the scratch tree sits. A scratch tree that is not inside any repository at all, the system temp directory among them, is not bounded this way, and it still reaches the workbenches in his home. Put scratch trees outside his profile entirely, `C:\dinah-scratch` or similar, and delete them when you are done; the repository bound narrows the hazard for a worktree, since every worktree here is itself a repository, but it does not change where you put one, because a worktree that ever stopped being a repository, or a scratch tree that was never one, would lose the protection the bound gives it and fall back on the profile rule alone. An agent that believed it was isolated read his live workbenches; another that thought it was in a scratch directory ran `dinah init` and wrote a workbench into an unrelated repository of his.

## The rules

Never operate on anything under `c:\users\paul\.dinah`, and never operate on the in-repo workbench at `.dinah/` in place. Copy it outside his profile when you want realistic data, and work on the copy.

Never run `dinah init` in a directory you did not create.

Never run a repair or migration against a live workbench. Copy first. This holds even when the repair is the thing under test, and especially then.

Verify isolation rather than assuming it: after a run, check that the file or directory you meant to leave alone is unchanged, and say in your handoff that you checked and what you found. Checking afterwards is the only version of this that proves anything.

Work in your worktree. The main checkout is the operator's own working tree, with the trunk branch checked out and his uncommitted work in it, and a destructive git command there is his to run, not yours.

## Why the bar is this high

An agent's mistake here is invisible to the person it happens to. He does not see the command, and the damage looks like his own tool misbehaving: a setting he never changed, a workbench that stops opening, an untracked file in a repository he was not working in. Every one of those has happened on this board, and each cost him time diagnosing something that was never his to diagnose.


## Scratch space is shared, so clean up only what is yours

Several cards run at once, and they all work outside the operator's profile in the same scratch area. That area is not yours alone.

Put everything you create under a directory named for your card, `C:\dinah-scratch\<card>-...`, including your worktree, your built binaries, and your fixtures. When you finish, delete that directory and nothing above it. Deleting the parent takes another card's worktree with it, and the agent holding it loses work it has already done and has to build it again.

The same applies to a temporary directory you point the build or the test suite at: name it for your card, and remove your own.
