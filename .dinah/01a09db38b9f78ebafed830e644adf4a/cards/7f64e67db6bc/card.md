---
title: a branch name carrying the letters git is enough for the guard to refuse the push
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
tier: workhorse
---
Found while implementing dinah-291. That card's branch is `dinah-291-the-guard-classifies-a-git-bash-path`, and pushing it was refused by the destructive-git guard. The branch name is an argument, not a command, and the letters that triggered the refusal are the ones spelling `git` inside `git-bash`. Quoting the refspec cleared it.

Branch names on this board come from card titles, by the `branch_pattern` directive every flow column carries. So any card whose title contains the word cannot be pushed by the ordinary spelling, and the agent that meets the refusal has no reason to suspect its own branch name. It reads a message about repository-mutating git and looks at the verb.

## Why the guard does this at all

The behaviour is deliberate in a neighbouring case and the hook's docstring explains it: an unquoted `-C` whose path carries a git component, as `git -C /home/x/git/repo commit`, is refused, because a path containing the token cannot be told apart from a second invocation by a scanner that reads text. The guard reads text on purpose, and that decision is sound and is not what this card disputes.

What this card reports is that the same reasoning has been applied to a token in argument position where no second invocation could be hiding, and the cost has changed now that branch names are generated from card titles rather than typed by a person who could rename them.

## What the fix has to settle

**Where the token is allowed to appear.** A refspec and a branch name are arguments to a verb the guard has already classified. Whether the scanner can tell that position apart from a position where an invocation could begin is the question, and the answer decides whether this is fixable without weakening the rule the docstring defends.

**Whether the refusal can name the real cause.** Today it reports the invocation it thinks it found. A reader whose branch name is the trigger is told nothing that would lead them to the branch name, which is why dinah-291's implementer worked around it rather than recognising it.

**What the workaround costs.** Quoting the refspec clears it, and that is a fine remedy for somebody who knows. It is not discoverable from the message, and the board's own instructions do not mention it.

## Related

dinah-291 repairs the path classifier in this same hook and is where this was found. dinah-325 covers the guard's fail-open behaviour. All three are the same file.
