# Dinah

## GitHub identity

Two GitHub accounts serve this repository. Paul owns `paulmooreparks`, and agents
publish their work through the `dinah-gh` bot account so that Paul can review and
approve a pull request he did not open himself.

Git itself always runs as `paulmooreparks`. The global setting
`credential.https://github.com.username` pins that account, so `git push` and
`git fetch` need no special handling and Git Credential Manager no longer asks
which account to use.

Pull requests are a separate matter, because `gh` authenticates with its own token
instead of going through the git credential helper. Create every agent-authored
pull request under the bot account by supplying its token for that one command:

```
GH_TOKEN="$(cat /c/Users/paul/source/repos/dinah/.dinah-gh-token)" gh pr create ...
```

The token file sits in the root checkout and is git-ignored, so it does not travel
into a worktree. Always reach for it by that absolute path. Do not copy it, print
it, or let its contents reach a log, a commit, or a comment.

Only pull request creation needs the bot token. Everything else, including
read-only `gh` calls, runs fine under Paul's own account.
