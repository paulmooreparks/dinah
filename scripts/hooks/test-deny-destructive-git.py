#!/usr/bin/env python3
r"""Cases the destructive-git guard must hold to.

Run it from anywhere: `python scripts/hooks/test-deny-destructive-git.py`.
It exits non-zero on the first disagreement and prints every case either
way, so a failure names itself rather than needing a debugger.

The table builds its own repository under a temporary directory, with a
main checkout and three linked worktrees, rather than asserting against
the operator's real one. That keeps the run reproducible on a machine
whose worktrees sit somewhere else, and it means the test never points a
destructive command at anything a person cares about.
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


# Every verb the guard refuses in the main checkout. Each one is run three
# ways: in the main checkout, where it must be denied; in a linked
# worktree, where it must be allowed; and with no working directory
# reported at all, where the guard has nothing to classify and fails
# closed.
MUTATING = [
    ("reset --hard", "git reset --hard origin/main"),
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

# Ordinary work in the operator's own checkout, which the guard has no
# business refusing. Every carve-out named in the deny set appears here,
# because an exemption nobody tests is an exemption that quietly closes.
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
    ("reset without --hard", "git reset HEAD~1"),
    ("restore --staged only", "git restore --staged seed.txt"),
    ("push --force-with-lease to a topic", "git push --force-with-lease origin topic"),
]


def cases(root, main, linked, spaced, nested):
    gone = os.path.join(main, ".claude", "worktrees", "deleted-out-from-under-us")
    backslashed = linked.replace("/", "\\") if os.name == "nt" else linked
    table = []

    for name, command in MUTATING:
        table.append(("%s in the main checkout" % name, command, main, DENY))
        table.append(("%s in a linked worktree" % name, command, linked, ALLOW))
        table.append(("%s with no cwd reported" % name, command, "", DENY))

    for name, command in ORDINARY:
        table.append(("%s in the main checkout" % name, command, main, ALLOW))

    table.extend([
        # The push the Implement stage runs carries a colon, so it is
        # refused in the checkout and allowed where that stage works.
        ("push HEAD to a full refname in a worktree", "git push origin HEAD:refs/heads/topic", linked, ALLOW),
        ("push HEAD to a full refname in the main checkout",
         "git push origin HEAD:refs/heads/topic", main, DENY),
        # A worktree nested inside the checkout is still a linked worktree.
        # The board forbids creating one there for an unrelated reason,
        # which is Dinah's workbench discovery, and enforcing that is not
        # this guard's job. It answers one question: is this the main
        # checkout?
        ("reset --hard in a nested worktree", "git reset --hard origin/main", nested, ALLOW),
        # An explicit -C decides the target, whatever the working directory is.
        ("-C main from a linked worktree", "git -C %s checkout -- ." % main, linked, DENY),
        ("-C linked worktree from main", "git -C %s stash pop" % linked, main, ALLOW),
        ("-C quoted path with a space", 'git -C "%s" clean -fdx' % main, linked, DENY),
        ("-C wins over an earlier cd", "cd %s && git -C %s stash pop" % (linked, main), linked, DENY),
        # Fail closed on anything unclassifiable.
        ("working directory no longer exists", "git clean -fdx", gone, DENY),
        ("target is not a git work tree", "git -C %s clean -fdx" % tempfile.gettempdir(), main, DENY),

        # A cd in the command being judged is the ordinary way an agent
        # reaches its worktree, because the shell's directory does not
        # persist between calls.
        ("cd to a worktree then stash pop", "cd %s && git stash pop" % linked, main, ALLOW),
        ("cd to the main checkout from a worktree", "cd %s && git stash pop" % main, linked, DENY),
        ("cd to a quoted path containing a space", 'cd "%s" && git stash pop' % spaced, main, ALLOW),
        ("cd to a path written with backslashes", "cd %s && git stash pop" % backslashed, main, ALLOW),
        ("a relative cd resolves against the payload cwd",
         "cd scratch/card-impl/wt && git stash pop", root, ALLOW),
        ("a relative cd onto the main checkout", "cd checkout && git stash pop", root, DENY),
        ("cd counts only as the first token of a segment",
         "echo cd %s && git stash pop" % linked, main, DENY),
        ("cd to an unexpanded variable", "cd $WORKTREE && git stash pop", main, DENY),
        ("cd with no argument", "cd && git stash pop", main, DENY),

        # A cd reaches a later invocation across these three separators
        # and across no others, because in the other three the git can run
        # without the cd having succeeded or runs in another process.
        ("cd carries across &&", "cd %s && git stash pop" % linked, main, ALLOW),
        ("cd carries across ;", "cd %s ; git stash pop" % linked, main, ALLOW),
        ("cd carries across a newline", "cd %s\ngit stash pop" % linked, main, ALLOW),
        ("cd does not carry across ||", "cd %s || git stash pop" % linked, main, DENY),
        ("cd does not carry across |", "cd %s | git stash pop" % linked, main, DENY),
        ("cd does not carry across &", "cd %s & git stash pop" % linked, main, DENY),
        # A redirection is not a separator, so the cd survives it.
        ("a redirection is not a separator", "cd %s 2>&1 && git stash pop" % linked, main, ALLOW),

        # Parentheses are a scope. A cd inside one reaches the rest of that
        # subshell and nothing after it, which is what keeps the second and
        # third cases below from being a way around the guard.
        ("a parenthesised cd and its git", "(cd %s && git stash pop)" % linked, main, ALLOW),
        ("a cd does not escape its parentheses", "(cd %s) && git reset --hard" % linked, main, DENY),
        ("a subshell does not cover what follows it",
         "(cd %s && git stash pop) && git reset --hard" % linked, main, DENY),
        ("a cd before a subshell reaches into it",
         "cd %s && (git stash pop)" % linked, main, ALLOW),

        # Quoted text is an argument rather than a directory. The guard
        # reads tokens, so a cd inside a commit message supplies nothing,
        # and a quoted path containing a space still resolves.
        ("a cd inside a quoted argument supplies no directory",
         'git commit -m "cd %s && note" && git reset --hard' % linked, main, DENY),
        ("prose quoting a destructive command is not one",
         'git log --oneline --grep "git clean -fdx"', main, ALLOW),
        # A heredoc body is data, so the same bypass through a different door.
        ("a cd inside a heredoc body supplies no directory",
         "git commit -F - <<EOF\ncd %s\nEOF\ngit reset --hard" % linked, main, DENY),
        ("a heredoc body does not hide a later command",
         "git log <<EOF\nnotes\nEOF\ncd %s && git stash pop" % linked, main, ALLOW),

        # Tokenising has to survive what agents actually type.
        ("a bare apostrophe tokenises and is judged",
         "git commit -m it's && git clean -fd", main, DENY),
        ("a bare apostrophe does not swallow a cd",
         "git log -m it's && cd %s && git stash pop" % linked, main, ALLOW),
        ("an unterminated quotation mark is unreadable and refused",
         'git commit -m "oops && git clean -fd', main, DENY),

        # Several invocations in one string are judged one at a time, and
        # one refusal refuses the command.
        ("several invocations, the last one in the main checkout",
         "cd %s && git stash pop && cd %s && git reset --hard" % (linked, main), main, DENY),
        ("several invocations, all in a worktree",
         "cd %s && git stash pop && git commit -m done" % linked, main, ALLOW),
        ("a later line is judged on its own",
         "git push origin topic\ngit log --oneline", main, ALLOW),
        # A backslash continuation is one real invocation and stays caught.
        ("continuation splits the flag", "git reset \\\n  --hard origin/main", main, DENY),
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
            print("%s %-52s want=%-5s got=%s" % (mark, name, want, got))
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
