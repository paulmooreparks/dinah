#!/usr/bin/env python3
"""Cases for deny-destructive-git.py. Run it from anywhere; it exits non-zero on a failure."""

import importlib.util
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("guard", os.path.join(HERE, "deny-destructive-git.py"))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)

MAIN = "C:/Users/paul/source/repos/dinah"
WT = "C:/dinah-scratch/card-1/wt"

# (command, payload cwd, refused?)
CASES = [
    # Destructive in the main checkout is refused, however the place is named.
    ("git reset --hard origin/main", MAIN, True),
    (f"git -C {MAIN} clean -fdx", WT, True),
    (f"cd {MAIN} && git stash", WT, True),
    ("cd /c/Users/paul/source/repos/dinah; git checkout -- .", WT, True),
    ("git rm card.go", MAIN + "/internal", True),
    ("Set-Location $env:REPO; git reset --hard", WT, True),
    (f"git --work-tree={MAIN} -C {WT} reset --hard", WT, True),
    ("git worktree remove", MAIN, True),
    # The same verbs in a worktree are allowed, however the worktree is named.
    (f"git -C {WT} reset --hard origin/main", MAIN, False),
    ("cd /c/dinah-scratch/card-1/wt && git stash", MAIN, False),
    (f"Set-Location {WT}; git clean -fd", MAIN, False),
    ("git checkout -- file.go", WT, False),
    (f"git -C {MAIN} worktree remove {WT}", MAIN, False),
    (f"git -C {MAIN} worktree remove {MAIN}/.claude/worktrees/agent-1", MAIN, False),
    # Reads and ordinary writes are never refused.
    ("git status --short", MAIN, False),
    ("git stash list", MAIN, False),
    ("git restore --staged file.go", MAIN, False),
    ("git rm --cached file.go", MAIN, False),
    (f"git -C C:/dinah-scratch/dinah-249-merge/wt status", MAIN, False),
    ("git log --oneline -- rm-fixture", MAIN, False),
    ("git commit -m 'a message'", MAIN, False),
    ("go test ./...", MAIN, False),
]


def main():
    failures = 0
    for command, cwd, refused in CASES:
        got = guard.decide(command, cwd, MAIN)
        ok = (got is not None) == refused
        failures += not ok
        print(("ok   " if ok else "FAIL ") + ("refuse " if refused else "allow  ") + command + ("" if ok else f"  -> {got}"))
    print(f"{len(CASES)} cases, {failures} failed")
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
