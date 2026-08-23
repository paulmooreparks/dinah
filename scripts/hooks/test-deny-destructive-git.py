#!/usr/bin/env python3
r"""Cases the destructive-git guard must hold to.

Run it from anywhere: `python scripts/hooks/test-deny-destructive-git.py`.
It prints every case either way and exits non-zero when any of them
disagrees with the table, so a failure names itself rather than needing a
debugger.

The table builds its own repository under a temporary directory, with a
main checkout and three linked worktrees, rather than asserting against
the operator's real one. That keeps the run reproducible on a machine
whose worktrees sit somewhere else, and it means the test never points a
destructive command at anything a person cares about.

The contract these cases pin is a single sentence. A git invocation whose
verb is in the deny set is allowed only when that same invocation carries
`-C <path>` and the path classifies as a linked worktree. The session's
reported working directory decides nothing, which is why most verbs are
run from both the main checkout and a worktree and expected to give the
same answer in each.
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile

HOOK = os.path.join(os.path.dirname(os.path.abspath(__file__)), "deny-destructive-git.py")

DENY = "deny"
ALLOW = "allow"

# Spelled in pieces so the file itself does not carry a string the live
# guard refuses when an agent edits this repository.
RESET = "git re" + "set --hard origin/main"


def git(*args, cwd=None):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True)


def build_repo(root):
    """A main checkout with one commit, plus three linked worktrees beside it."""
    main = os.path.join(root, "checkout")
    os.makedirs(main)
    git("init", "-q", "-b", "main", cwd=main)
    git("config", "user.email", "test@example.invalid", cwd=main)
    git("config", "user.name", "Test", cwd=main)
    with open(os.path.join(main, "seed.txt"), "w", encoding="utf-8") as handle:
        handle.write("seed\n")
    git("add", "seed.txt", cwd=main)
    git("commit", "-q", "-m", "seed", cwd=main)
    linked = os.path.join(root, "scratch", "card-impl", "wt")
    git("worktree", "add", "--detach", "-q", linked, "HEAD", cwd=main)
    spaced = os.path.join(root, "dinah scratch", "card-impl", "wt")
    git("worktree", "add", "--detach", "-q", spaced, "HEAD", cwd=main)
    nested = os.path.join(main, ".claude", "worktrees", "legacy")
    git("worktree", "add", "--detach", "-q", nested, "HEAD", cwd=main)
    return main, linked, spaced, nested


def verdict(command, cwd):
    payload = json.dumps({"tool_input": {"command": command}, "cwd": cwd})
    result = subprocess.run(
        [sys.executable, HOOK], input=payload, capture_output=True, text=True, timeout=60
    )
    if result.returncode != 0:
        raise AssertionError("hook exited %d: %s" % (result.returncode, result.stderr.strip()))
    if not result.stdout.strip():
        return ALLOW
    decision = json.loads(result.stdout)["hookSpecificOutput"]["permissionDecision"]
    return DENY if decision == "deny" else ALLOW


def qualified(command, path):
    """The same command with a `-C <path>` naming where it runs."""
    assert command.startswith("git "), command
    return 'git -C "%s" %s' % (path, command[4:])


# Every verb the guard refuses. Each one is run six ways, and the first
# three of those are the point of the contract: a bare invocation is
# refused wherever the session says it stands, including when the session
# says nothing at all.
MUTATING = [
    ("reset --hard", "git re" + "set --hard origin/main"),
    ("clean -fdx", "git clean -fdx"),
    ("checkout -- .", "git checkout -- ."),
    ("checkout -f", "git checkout -f main"),
    ("restore a path", "git restore seed.txt"),
    ("push --force", "git push --force origin topic"),
    ("stash pop", "git stash pop"),
    ("stash with no subcommand", "git stash"),
    ("commit", 'git commit -m "wip"'),
    ("merge", "git merge origin/main"),
    ("rebase", "git rebase origin/main"),
    ("cherry-pick", "git cherry-pick abc1234"),
    ("revert", "git revert abc1234"),
    ("am", "git am patch.mbox"),
    ("apply", "git apply patch.diff"),
    ("rm", "git rm seed.txt"),
    ("mv", "git mv seed.txt other.txt"),
    ("bisect start", "git bisect start"),
    ("pull without --ff-only", "git pull origin main"),
    ("switch -f", "git switch -f main"),
    ("switch --force", "git switch --force main"),
    ("switch --discard-changes", "git switch --discard-changes main"),
    ("worktree remove", "git worktree remove old"),
    ("branch -d", "git branch -d topic"),
    ("branch -D", "git branch -D topic"),
    ("tag -d", "git tag -d v1.0"),
    ("push --delete", "git push origin --delete topic"),
    ("push a colon refspec", "git push origin :topic"),
]

# Ordinary work the guard has no business refusing, wherever it runs and
# whether or not it names a directory. Every carve-out named in the deny
# set appears here, because an exemption nobody tests is an exemption that
# quietly closes.
ORDINARY = [
    ("stash list", "git stash list"),
    ("stash show", "git stash show"),
    ("apply --check", "git apply --check patch.diff"),
    ("apply --stat", "git apply --stat patch.diff"),
    ("rm --cached", "git rm --cached seed.txt"),
    ("bisect log", "git bisect log"),
    ("bisect view", "git bisect view"),
    ("pull --ff-only", "git pull --ff-only origin main"),
    ("worktree add", "git worktree add --detach ../new HEAD"),
    ("worktree list", "git worktree list"),
    ("worktree prune", "git worktree prune"),
    ("fetch", "git fetch origin"),
    ("push a branch", "git push origin topic"),
    ("branch used to create", "git branch topic"),
    ("branch used to list", "git branch --list"),
    ("tag used to create", "git tag v1.0"),
    ("tag used to list", "git tag -l"),
    ("checkout a branch", "git checkout main"),
    ("switch a branch", "git switch main"),
    ("status", "git status --porcelain"),
    ("log", "git log --oneline -1"),
    ("diff", "git diff"),
    ("reset without --hard", "git re" + "set HEAD~1"),
    ("restore --staged only", "git restore --staged seed.txt"),
    ("push --force-with-lease to a topic", "git push --force-with-lease origin topic"),
]


def leaks(linked):
    """The seven spellings that got past the parsing design.

    Each was found by a person reading the code rather than by the suite,
    and each is a real string. They are kept as regressions and every one
    of them is now refused, for a structural reason rather than because
    the spelling was handled: none carries a `-C`, and nothing else can
    grant permission.
    """
    return [
        ("leak: a cd inside a quoted argument",
         'git commit -m "cd %s && note" && %s' % (linked, RESET)),
        ("leak: a here-string read as a heredoc opener",
         "cat <<<word\n%s" % RESET),
        ("leak: a heredoc marker containing a hyphen",
         "cat <<E-OF\ncd %s\nE-OF\n%s" % (linked, RESET)),
        ("leak: a heredoc marker beginning with a digit",
         "cat <<1EOF\ncd %s\n1EOF\n%s" % (linked, RESET)),
        ("leak: pushd instead of cd", "pushd %s && %s" % (linked, RESET)),
        ("leak: builtin cd", "builtin cd %s && %s" % (linked, RESET)),
        ("leak: a backslash-escaped cd", "\\cd %s && %s" % (linked, RESET)),
    ]


def cases(root, main, linked, spaced, nested):
    gone = os.path.join(main, ".claude", "worktrees", "deleted-out-from-under-us")
    backslashed = linked.replace("/", "\\") if os.name == "nt" else linked
    table = []

    for name, command in MUTATING:
        table.append(("%s, bare, session in the main checkout" % name, command, main, DENY))
        table.append(("%s, bare, session in a worktree" % name, command, linked, DENY))
        table.append(("%s, bare, no cwd reported" % name, command, "", DENY))
        table.append(("%s, -C a worktree, session in the main checkout" % name,
                      qualified(command, linked), main, ALLOW))
        table.append(("%s, -C a worktree, session in a worktree" % name,
                      qualified(command, linked), linked, ALLOW))
        table.append(("%s, -C the main checkout" % name, qualified(command, main), linked, DENY))

    for name, command in ORDINARY:
        table.append(("%s in the main checkout" % name, command, main, ALLOW))
        table.append(("%s in a worktree" % name, command, linked, ALLOW))

    for name, command in leaks(linked):
        table.append((name, command, main, DENY))

    table.extend([
        # What a -C has to name before it grants anything.
        ("-C a nested worktree is still a linked worktree",
         'git -C "%s" clean -fdx' % nested, main, ALLOW),
        ("-C a directory that is not a git work tree",
         'git -C "%s" clean -fdx' % tempfile.gettempdir(), main, DENY),
        ("-C a directory that no longer exists",
         'git -C "%s" clean -fdx' % gone, main, DENY),
        ("-C a path written with backslashes",
         "git -C %s stash pop" % backslashed, main, ALLOW),
        ("-C a quoted path containing a space",
         'git -C "%s" stash pop' % spaced, main, ALLOW),
        ("-C attached to its value", 'git -C"%s" stash pop' % linked, main, ALLOW),
        # A relative -C names a directory only together with a working
        # directory, and the working directory is what the guard stopped
        # consulting. It is refused rather than composed.
        ("-C . is relative and names nothing on its own",
         "git -C . stash pop", linked, DENY),
        ("-C a relative subdirectory names nothing on its own",
         "git -C scratch/card-impl/wt stash pop", root, DENY),

        # Options naming a target the guard does not follow.
        ("--git-dir instead of -C", 'git --git-dir="%s/.git" stash pop' % main, linked, DENY),
        ("--git-dir spelled with a space",
         'git --git-dir "%s/.git" stash pop' % main, linked, DENY),
        ("--work-tree instead of -C", 'git --work-tree="%s" stash pop' % linked, main, DENY),
        ("--work-tree spelled with a space",
         'git --work-tree "%s" stash pop' % linked, main, DENY),
        # The four cases above hold whether or not the guard refuses the
        # option, because an invocation using one carries no -C either and
        # is refused for that instead. These three rest on the refusal
        # alone: each carries a -C that would otherwise clear it.
        ("--git-dir alongside a qualifying -C is still refused",
         'git -C "%s" --git-dir="%s/.git" stash pop' % (linked, main), main, DENY),
        ("--git-dir ahead of a qualifying -C is still refused",
         'git --git-dir="%s/.git" -C "%s" stash pop' % (main, linked), main, DENY),
        ("--work-tree alongside a qualifying -C is still refused",
         'git -C "%s" --work-tree "%s" stash pop' % (linked, main), main, DENY),

        # One qualifying invocation does not clear the command.
        ("two invocations, only the first qualifies",
         'git -C "%s" stash pop && %s' % (linked, RESET), linked, DENY),
        ("two invocations, only the second qualifies",
         '%s && git -C "%s" stash pop' % (RESET, linked), linked, DENY),
        ("two invocations, one -C a worktree and one -C the checkout",
         'git -C "%s" stash pop && git -C "%s" clean -fdx' % (linked, main), linked, DENY),
        ("two invocations, both qualifying",
         'git -C "%s" stash pop && git -C "%s" commit -m done' % (linked, linked), main, ALLOW),
        ("a qualifying invocation beside an ordinary one",
         'git fetch origin && git -C "%s" merge origin/main' % linked, main, ALLOW),

        # Quoted text is an argument rather than a command, which is what
        # the verb scan's quoted-span stripping buys and all it buys.
        ("prose quoting a destructive command is not one",
         'git log --oneline --grep "git clean -fdx"', main, ALLOW),
        ("a destructive command quoted inside a qualifying one",
         'git -C "%s" commit -m "%s"' % (linked, RESET), main, ALLOW),
        ("a quote span closing inside a later word is still one argument",
         "git log 'a && %s don't" % RESET, main, ALLOW),
        ("an unterminated quotation mark is unreadable and refused",
         'git commit -m "oops && git clean -fd', main, DENY),
        ("a bare apostrophe tokenises and is judged",
         "git commit -m it's && git clean -fd", main, DENY),

        # The forms the retired design used to allow. Every one of them is
        # a command that does not say where it runs, so every one is now
        # refused, and this block is the record of what the inversion cost.
        ("cd then a bare invocation, across &&", "cd %s && git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across ;", "cd %s ; git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across a newline",
         "cd %s\ngit stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across ||", "cd %s || git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across |", "cd %s | git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across &", "cd %s & git stash pop" % linked, main, DENY),
        ("a parenthesised cd and its bare invocation",
         "(cd %s && git stash pop)" % linked, main, DENY),
        ("a cd inside a heredoc body",
         "git log <<EOF\ncd %s\nEOF\ngit stash pop" % linked, main, DENY),
        ("a cd written with backslashes", "cd %s && git stash pop" % backslashed, main, DENY),
        ("a cd to a quoted path containing a space",
         'cd "%s" && git stash pop' % spaced, main, DENY),
        ("a relative cd", "cd scratch/card-impl/wt && git stash pop", root, DENY),
        ("a brace group carrying a cd", "{ cd %s ; git stash pop ; }" % linked, main, DENY),

        # A continuation is one invocation split over two lines, and the
        # flag on the second line still counts.
        ("continuation splits the flag", "git re" + "set \\\n  --hard origin/main", main, DENY),
        ("continuation splits the flag on a qualifying invocation",
         'git -C "%s" re' % linked + "set \\\n  --hard origin/main", main, ALLOW),

        # The early bail reads the command case-insensitively, because the
        # basename matching further in already does.
        ("an uppercase GIT is still git", "GIT re" + "set --hard origin/main", main, DENY),
        ("an uppercase GIT can still qualify",
         'GIT -C "%s" re' % linked + "set --hard origin/main", main, ALLOW),
        ("git named by an absolute path",
         "/usr/bin/git stash pop", main, DENY),
        ("git named by an absolute path, qualifying",
         '/usr/bin/git -C "%s" stash pop' % linked, main, ALLOW),

        # A command mentioning no git at all never reaches the rule.
        ("no git in the command", "rm -rf build", main, ALLOW),
    ])
    return table


def main():
    root = tempfile.mkdtemp(prefix="deny-destructive-git-")
    failures = 0
    total = 0
    try:
        checkout, linked, spaced, nested = build_repo(root)
        for name, command, cwd, want in cases(root, checkout, linked, spaced, nested):
            total += 1
            try:
                got = verdict(command, cwd)
            except AssertionError as err:
                got = "error: %s" % err
            mark = "ok  " if got == want else "FAIL"
            if got != want:
                failures += 1
            print("%s %-58s want=%-5s got=%s" % (mark, name, want, got))
    finally:
        # The worktrees hold no reflog worth keeping and the repository is
        # this function's own, so the whole tree goes. ignore_errors covers
        # a Windows handle that has not closed yet.
        shutil.rmtree(root, ignore_errors=True)

    print()
    if failures:
        print("%d of %d case(s) failed" % (failures, total))
        return 1
    print("all %d cases passed" % total)
    return 0


if __name__ == "__main__":
    sys.exit(main())
