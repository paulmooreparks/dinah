#!/usr/bin/env python3
r"""PreToolUse guard: a mutating git command has to name the worktree it runs in.

Reads the Claude Code hook payload on stdin. For each git invocation in
the command whose verb is in the deny set, the invocation is allowed only
when it carries `-C <path>` itself and that path classifies as a linked
worktree. Everything else is refused.

The guard does not work out where a command will run. An earlier version
did, and it parsed separators, quoting, heredocs, subshells and the `cd`
builtin to do it. Seven leaks were found in that design across two review
cycles, every one by a person reading the code and none by a suite that
reached 167 cases, and each was a spelling nobody had enumerated: a
here-string, a heredoc marker containing a hyphen, a heredoc marker
beginning with a digit, `pushd`, `builtin cd`, `\cd`, and the
`--work-tree` and `--git-dir` options. Nothing in that design bounded how
many spellings remained.

So the question changed. The guard no longer asks where a command will
run; it asks whether the command says where it runs. A spelling the guard
has never heard of cannot help a command through, because nothing the
command says can grant permission except the one thing the guard reads.

The payload's `cwd` is not consulted, and that is a choice rather than an
oversight. A conditional form, requiring `-C` only when the session
reports itself standing in the operator's checkout, reopens a leak this
guard already had: a session reported as standing in a worktree would be
allowed a bare command, and `cd <the checkout> && git reset --hard` from
such a session runs the reset in the checkout.

An invocation carrying `--git-dir` or `--work-tree` is refused rather
than analysed. Both name a target the guard does not follow, so the
honest answer is that the invocation has not said where it runs in the
form the guard reads. A relative `-C` is refused for the same reason: it
names a directory only in combination with a working directory, and the
working directory is exactly what the guard has stopped consulting.

Is a named directory a linked worktree? `git rev-parse --git-dir
--git-common-dir` is the documented discriminator. The two answers differ
in a linked worktree, where the per-worktree git dir sits under
`<common>/worktrees/<name>`, and match in the main worktree. Nothing here
depends on where a worktree is created, so the guard stays correct if the
convention moves.

The deny set is chosen rather than inherited. What is refused: reset
--hard/--merge/--keep, clean with -f/-d/-x, checkout -- / -f, restore of
the working tree, push --force, push --force-with-lease at main or
master, stash other than list and show, commit, merge, rebase,
cherry-pick, revert, am, apply without --check or --stat, rm without
--cached, mv, bisect other than log and view, switch with
-f/--force/--discard-changes, pull without --ff-only, worktree remove,
branch -d/-D, tag -d, and push with --delete or a colon refspec. The ref
deletions are refused because refs are shared across every worktree of
the repository, so deleting one reaches into whatever another agent is
standing on.

Outside the deny set entirely, so a bare form of each is unaffected
wherever it runs: worktree add, list, and prune; plain checkout and
switch of a branch; fetch; push of a branch; branch and tag used to
create or list; stash list and show; and every read. `worktree prune`
clears the administrative record for a directory that is already gone,
skips a locked worktree, and destroys no commits, and it is the only way
to finish the cleanup the board's safety document requires.

What this costs is stated rather than discovered. A bare `git commit`
typed inside a worktree is refused, including by the operator in his own
session. The board's own agents are unaffected, because the
explicit-path discipline dinah-228 installs already requires `git -C
<worktree>` on every git command from every stage.

One thing this guard does not cover and never did: a command that
changes directory and then runs something other than git. It reads git
commands.
"""

import json
import os
import re
import shlex
import subprocess
import sys


GIT_TIMEOUT = 5

# A word carrying `git` at all, used only to decide whether a command that
# will not tokenise is worth refusing.
GIT_WORD = re.compile(r"\bgit\b", re.IGNORECASE)

# Options naming a target the guard does not follow.
UNFOLLOWED = ("--git-dir", "--work-tree")

# Options consuming the token after them, skipped while hunting for the
# subcommand. `-C` is read rather than skipped and so is not listed here.
TAKES_VALUE = ("-c", "--namespace", "--exec-path", "--super-prefix") + UNFOLLOWED


def unquote(token):
    """Strip one matched surrounding pair of quotation marks.

    Non-POSIX tokenising leaves the marks attached to the token, and a
    path is wanted rather than its spelling.
    """
    if len(token) >= 2 and token[0] == token[-1] and token[0] in "\"'":
        return token[1:-1]
    return token


def git_slices(tokens):
    """Each git invocation in the command, as its own token list.

    A slice runs from one `git` word to the next, so an option belonging
    to the second invocation cannot be read as part of the first. Quoted
    text is one token and its interior is never a `git` word, which is
    what keeps a commit message mentioning a command from being read as
    one.
    """
    starts = [position for position, token in enumerate(tokens)
              if os.path.basename(unquote(token)).lower() in ("git", "git.exe")]
    slices = []
    for order, start in enumerate(starts):
        end = starts[order + 1] if order + 1 < len(starts) else len(tokens)
        slices.append(tokens[start:end])
    return slices


def read_invocation(invocation):
    """The `-C` path, any unfollowed option, the subcommand, and its arguments."""
    directory = None
    unfollowed = None
    index = 1
    while index < len(invocation):
        token = unquote(invocation[index])
        if token == "-C" and index + 1 < len(invocation):
            directory = unquote(invocation[index + 1])
            index += 2
            continue
        if token.startswith("-C") and len(token) > 2:
            directory = unquote(token[2:])
            index += 1
            continue
        if unfollowed is None:
            for option in UNFOLLOWED:
                if token == option or token.startswith(option + "="):
                    unfollowed = option
                    break
        if token in TAKES_VALUE:
            index += 2
            continue
        if token.startswith("-"):
            index += 1
            continue
        break
    if index >= len(invocation):
        return directory, unfollowed, None, []
    subcommand = unquote(invocation[index])
    arguments = [unquote(token) for token in invocation[index + 1:]]
    return directory, unfollowed, subcommand, arguments


def has(arguments, *names):
    return any(argument in names for argument in arguments)


def short(arguments, letters):
    """True when a bundled short flag carries one of `letters`."""
    for argument in arguments:
        if argument.startswith("-") and not argument.startswith("--") and len(argument) > 1:
            if any(letter in argument[1:] for letter in letters):
                return True
    return False


def colon_refspec(argument):
    if argument.startswith("-") or ":" not in argument:
        return False
    # A remote spelled as a URL carries a colon and is not a refspec.
    return "://" not in argument and "@" not in argument


def denied(subcommand, arguments):
    """The label of the rule a git invocation breaks, or None."""
    if subcommand is None:
        return None
    if subcommand == "reset" and has(arguments, "--hard", "--merge", "--keep"):
        return "git reset --hard/--merge/--keep"
    if subcommand == "clean" and (short(arguments, "fdxX") or has(arguments, "--force")):
        return "git clean -f/-d/-x"
    if subcommand == "checkout" and ("--" in arguments or short(arguments, "f") or has(arguments, "--force")):
        return "git checkout -- / -f"
    if subcommand == "restore" and not (has(arguments, "--staged") and not has(arguments, "--worktree")):
        return "git restore (working tree)"
    if subcommand == "switch" and (short(arguments, "f") or has(arguments, "--force", "--discard-changes")):
        return "git switch -f/--force/--discard-changes"
    if subcommand == "push":
        if has(arguments, "--force") or short(arguments, "f"):
            return "git push --force"
        if any(argument.startswith("--force-with-lease") for argument in arguments):
            if any(re.search(r"\b(main|master)\b", argument) for argument in arguments):
                return "git push --force-with-lease to main/master"
        if has(arguments, "--delete") or any(colon_refspec(argument) for argument in arguments):
            return "git push --delete / colon refspec"
        return None
    if subcommand == "stash" and (not arguments or arguments[0] not in ("list", "show")):
        return "git stash (mutating)"
    if subcommand == "commit":
        return "git commit"
    if subcommand in ("merge", "rebase", "cherry-pick", "revert", "am"):
        return "git " + subcommand
    if subcommand == "apply" and not has(arguments, "--check", "--stat"):
        return "git apply"
    if subcommand == "rm" and not has(arguments, "--cached"):
        return "git rm"
    if subcommand == "mv":
        return "git mv"
    if subcommand == "bisect" and (not arguments or arguments[0] not in ("log", "view")):
        return "git bisect"
    if subcommand == "pull" and not has(arguments, "--ff-only"):
        return "git pull (may merge)"
    if subcommand == "worktree" and arguments and arguments[0] == "remove":
        return "git worktree remove"
    if subcommand == "branch" and (has(arguments, "--delete") or short(arguments, "dD")):
        return "git branch -d/-D"
    if subcommand == "tag" and (has(arguments, "--delete") or short(arguments, "d")):
        return "git tag -d"
    return None


def resolve(path, base):
    """Absolute, symlink-resolved form of a path that may be relative."""
    if not os.path.isabs(path):
        path = os.path.join(base, path)
    return os.path.normcase(os.path.realpath(path))


def worktree_kind(target):
    """Classify an absolute directory as "linked", "main", or "unknown".

    Asks git for both the per-worktree git dir and the common one. They
    differ in a linked worktree and match in the main worktree, which is
    what `--git-common-dir` is documented to express. "unknown" covers a
    directory that is not a work tree, a git that will not run, and any
    answer this cannot read; every one of those fails closed at the caller.
    """
    try:
        result = subprocess.run(
            ["git", "-C", target, "rev-parse", "--git-dir", "--git-common-dir"],
            capture_output=True,
            text=True,
            timeout=GIT_TIMEOUT,
        )
    except Exception:
        return "unknown"
    if result.returncode != 0:
        return "unknown"
    lines = [line.strip() for line in result.stdout.splitlines() if line.strip()]
    if len(lines) != 2:
        return "unknown"
    git_dir, common_dir = lines
    # git answers relative to the directory it was pointed at, so both are
    # resolved against that directory rather than against this process's.
    return "linked" if resolve(git_dir, target) != resolve(common_dir, target) else "main"


def offender(command):
    """The first invocation the guard will not clear, or None when all are clear.

    Returns `(label, fault)`, where the label names the rule and the
    fault says why the invocation did not earn its permission.
    """
    try:
        tokens = shlex.split(command, posix=False)
    except ValueError:
        # An unterminated quotation mark leaves the command unreadable,
        # and a hook that cannot read a command cannot clear it.
        if GIT_WORD.search(command):
            return "unreadable command", "the command does not tokenise"
        return None
    for invocation in git_slices(tokens):
        directory, unfollowed, subcommand, arguments = read_invocation(invocation)
        label = denied(subcommand, arguments)
        if not label:
            continue
        if unfollowed:
            return label, "%s names a target this guard does not follow" % unfollowed
        if directory is None:
            return label, "the invocation carries no -C"
        if not os.path.isabs(directory):
            return label, "-C %s is relative, so it names no directory on its own" % directory
        kind = worktree_kind(directory)
        if kind == "main":
            return label, "-C %s is the main checkout" % directory
        if kind != "linked":
            return label, "-C %s is not a git worktree" % directory
    return None


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return  # unparseable payload: stay out of the way

    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command") or ""
    if "git" not in command.lower():
        return

    refusal = offender(command)
    if refusal is None:
        return
    matched, fault = refusal

    reason = (
        "Blocked repository-mutating git ({0}), because {1}. A command running this "
        "class of git verb has to name its worktree on the invocation itself, as "
        "git -C <worktree> ... , and nothing else grants it permission. The shell's "
        "working directory does not persist between calls: it resets to the session's "
        "primary directory, which is the operator's checkout, so a command that does "
        "not say where it runs runs there. On this repository worktrees belong under "
        "C:\\dinah-scratch\\, never inside the checkout, because Dinah's workbench "
        "discovery climbs to the drive root and a nested worktree reaches the "
        "operator's live data. Create one with git -C <repo> worktree add --detach "
        "C:/dinah-scratch/<card>-<stage>/wt <ref>. If this command really does belong "
        "in the main checkout, it is the operator's to run, or the operator can "
        "disable this guard via /hooks (.claude/settings.json)."
    ).format(matched, fault)

    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }))


if __name__ == "__main__":
    main()
