#!/usr/bin/env python3
r"""Cases the destructive-git guard must hold to.

Run it from anywhere: `python scripts/hooks/test-deny-destructive-git.py`.
It exits non-zero on the first disagreement and prints every case either
way, so a failure names itself rather than needing a debugger.

The table builds its own repository under a temporary directory, with a
main checkout and a linked worktree, rather than asserting against the
operator's real one. That keeps the run reproducible on a machine whose
worktrees sit somewhere else, and it means the test never points a
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
    """A main checkout with one commit, plus a linked worktree beside it."""
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
    nested = os.path.join(main, ".claude", "worktrees", "legacy")
    git("worktree", "add", "--detach", "-q", nested, "HEAD", cwd=main)
    return main, linked, nested


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


def cases(main, linked, nested):
    gone = os.path.join(main, ".claude", "worktrees", "deleted-out-from-under-us")
    return [
        # The guard's whole point: the operator's own checkout is protected.
        ("reset --hard in the main checkout", "git reset --hard origin/main", main, DENY),
        ("clean -fdx in the main checkout", "git clean -fdx", main, DENY),
        ("checkout -- . in the main checkout", "git checkout -- .", main, DENY),
        ("push --force from the main checkout", "git push --force origin topic", main, DENY),
        # A linked worktree is where this work belongs, wherever it sits. The
        # second case is the one the path-prefix test used to get wrong: a
        # worktree outside the repository was denied for being outside it.
        ("reset --hard in a linked worktree", "git reset --hard origin/main", linked, ALLOW),
        ("clean -fdx in a linked worktree", "git clean -fdx", linked, ALLOW),
        ("push --force from a linked worktree", "git push --force origin topic", linked, ALLOW),
        # A worktree nested inside the checkout is still a linked worktree.
        # The board forbids creating one there for an unrelated reason, which
        # is Dinah's workbench discovery, and enforcing that is not this
        # guard's job. It answers one question: is this the main checkout?
        ("reset --hard in a nested worktree", "git reset --hard origin/main", nested, ALLOW),
        # An explicit -C decides the target, whatever the working directory is.
        ("-C main from a linked worktree", "git -C %s checkout -- ." % main, linked, DENY),
        ("-C linked worktree from main", "git -C %s checkout -- ." % linked, main, ALLOW),
        ('-C quoted path with a space', 'git -C "%s" clean -fdx' % main, linked, DENY),
        # Fail closed on anything unclassifiable.
        ("no working directory reported", "git clean -fdx", "", DENY),
        ("working directory no longer exists", "git clean -fdx", gone, DENY),
        ("target is not a git work tree", "git -C %s clean -fdx" % tempfile.gettempdir(), main, DENY),
        # Nothing outside the deny set is this hook's business.
        ("status in the main checkout", "git status --porcelain", main, ALLOW),
        ("reset without --hard", "git reset HEAD~1", main, ALLOW),
        ("restore --staged only", "git restore --staged file.txt", main, ALLOW),
        ("push --force-with-lease to a topic", "git push --force-with-lease origin topic", main, ALLOW),
        ("push --force-with-lease to main", "git push --force-with-lease origin main", main, DENY),
        # Prose that merely contains a destructive phrase is not a command.
        ("commit message quoting a reset", 'git commit -m "git reset --hard notes"', main, ALLOW),
        # A later line of a script is a separate command, not part of the first.
        ("unrelated second line", "git push origin topic\ngit worktree remove --force old", main, ALLOW),
        # A backslash continuation is one real invocation and stays caught.
        ("continuation splits the flag", "git reset \\\n  --hard origin/main", main, DENY),
    ]


def main():
    root = tempfile.mkdtemp(prefix="deny-destructive-git-")
    failures = 0
    try:
        checkout, linked, nested = build_repo(root)
        for name, command, cwd, want in cases(checkout, linked, nested):
            try:
                got = verdict(command, cwd)
            except AssertionError as err:
                got = "error: %s" % err
            mark = "ok  " if got == want else "FAIL"
            if got != want:
                failures += 1
            print("%s %-40s want=%-5s got=%s" % (mark, name, want, got))
    finally:
        # The worktrees hold no reflog worth keeping and the repository is
        # this function's own, so the whole tree goes. ignore_errors covers
        # a Windows handle that has not closed yet.
        shutil.rmtree(root, ignore_errors=True)

    print()
    if failures:
        print("%d case(s) failed" % failures)
        return 1
    print("all %d cases passed" % len(cases("", "", "")))
    return 0


if __name__ == "__main__":
    sys.exit(main())
