#!/usr/bin/env python3
"""PreToolUse guard: deny destructive git commands against the main checkout.

Motivation (andon-650): worktree-isolated subagents twice ran git state
mutations (a checkout and a reset --hard) whose Bash cwd was the MAIN
checkout, not their worktree, and the harness worktree guard did not fire.
Both happened to be benign; this hook makes the failure mode deterministic
instead of lucky.

Reads the Claude Code hook payload on stdin. Denies when the command
matches a destructive git pattern AND the effective target is the main
checkout. Fail-closed: a target this hook cannot classify counts as the
main checkout.

Deny set (deliberately narrow):
  - git reset --hard / --merge / --keep
  - git clean with -f/-d/-x style flags
  - git checkout -- <paths> (working-tree discard) and git checkout -f
  - git restore (working-tree discard without --staged-only usage)
  - git push --force / -f, and --force-with-lease when the refspec
    mentions main or master

Allow: anything else, and any denied pattern whose effective target is a
linked worktree.

The worktree test asks git rather than matching a path prefix. An earlier
version allowed a destructive command exactly when the path contained
`.claude/worktrees/`, which encoded one directory convention into a guard
whose actual question is "is this the operator's own checkout?". That
convention has since been reversed: this repository's worktrees belong
under `C:\\dinah-scratch\\`, because Dinah's workbench discovery climbs
from the working directory to the drive root, so a worktree nested inside
the repository sits below the repository's own `.dinah/` and reaches the
operator's live data. The old test denied every command in the location
the board mandates and trusted the one location the board forbids.

`git rev-parse --git-dir --git-common-dir` is the documented
discriminator. The two answers differ in a linked worktree, where the
per-worktree git dir sits under `<common>/worktrees/<name>`, and match in
the main worktree. Nothing here depends on where a worktree is created,
so the guard stays correct if the convention moves again.
"""

import json
import os
import re
import shlex
import subprocess
import sys


# A newline separates commands exactly as `;` does, so it belongs in every
# negated class below. Without it a script whose first line runs `git push`
# and whose fifth line runs an unrelated `git worktree remove --force` reads
# as one invocation and is denied, which is a false positive that teaches
# agents to work around the guard rather than respect it. Observed
# 2026-07-31. Genuine multi-line invocations use a backslash continuation,
# and join_continuations folds those back before this runs, so nothing real
# escapes by spanning lines.
DESTRUCTIVE = [
    (re.compile(r"\bgit\b[^|;&\n]*\breset\b[^|;&\n]*(--hard|--merge|--keep)\b"), "git reset --hard/--merge/--keep"),
    (re.compile(r"\bgit\b[^|;&\n]*\bclean\b[^|;&\n]*\s-[a-z]*[fdx]"), "git clean -f/-d/-x"),
    (re.compile(r"\bgit\b[^|;&\n]*\bcheckout\b[^|;&\n]*(\s--\s|\s-f\b|\s--force\b)"), "git checkout -- / -f"),
    (re.compile(r"\bgit\b[^|;&\n]*\brestore\b(?![^|;&\n]*--staged\b(?![^|;&\n]*--worktree\b))"), "git restore (working tree)"),
    (re.compile(r"\bgit\b[^|;&\n]*\bpush\b[^|;&\n]*(\s--force(?!-with-lease)\b|\s-f\b)"), "git push --force"),
    (re.compile(r"\bgit\b[^|;&\n]*\bpush\b[^|;&\n]*--force-with-lease[^|;&\n]*\b(main|master)\b"), "git push --force-with-lease to main/master"),
]

CONTINUATION = re.compile(r"\\[ \t]*\r?\n")

C_FLAG = re.compile(r"\bgit\s+-C\s+(\"[^\"]+\"|'[^']+'|\S+)")

QUOTED = re.compile(r"\"[^\"]*\"|'[^']*'")

GIT_TIMEOUT = 5


def join_continuations(command):
    """Fold shell line-continuations back onto one line.

    The destructive patterns stop at a newline so that two commands in one
    script are not read as one. A backslash continuation is the single case
    where one real invocation legitimately spans lines, so it is rejoined
    first and stays detectable: `git reset \\` on one line followed by
    `--hard` on the next is still `git reset --hard`."""
    return CONTINUATION.sub(" ", command)


def strip_quoted(command):
    """Remove quoted spans so prose (commit messages, heredoc text, echo
    strings) cannot false-positive the destructive patterns. Real
    destructive invocations carry their flags outside quotes. An unmatched
    trailing quote leaves the remainder in place, which errs toward
    matching (fail-closed)."""
    return QUOTED.sub(" ", command)


def resolve(path, base):
    """Absolute, symlink-resolved form of a path that may be relative."""
    if not os.path.isabs(path):
        path = os.path.join(base or os.getcwd(), path)
    return os.path.normcase(os.path.realpath(path))


def worktree_kind(target, base):
    """Classify a directory as "linked", "main", or "unknown".

    Asks git for both the per-worktree git dir and the common one. They
    differ in a linked worktree and match in the main worktree, which is
    what `--git-common-dir` is documented to express. "unknown" covers a
    directory that is not a work tree, a git that will not run, and any
    answer this cannot read; every one of those fails closed at the caller.
    """
    where = target if os.path.isabs(target) else os.path.join(base or os.getcwd(), target)
    try:
        result = subprocess.run(
            ["git", "-C", where, "rev-parse", "--git-dir", "--git-common-dir"],
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
    return "linked" if resolve(git_dir, where) != resolve(common_dir, where) else "main"


def targets(command, cwd):
    """The directories a command will act on.

    An explicit `-C` wins, and a command may carry more than one. With no
    `-C` the session's working directory is the target. An empty cwd is
    reported as no target at all, which the caller fails closed on."""
    found = [shlex.split(m.group(1))[0] if m.group(1)[:1] in "\"'" else m.group(1)
             for m in C_FLAG.finditer(command)]
    if found:
        return found
    return [cwd] if cwd else []


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return  # unparseable payload: stay out of the way

    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command") or ""
    if "git" not in command:
        return

    scannable = strip_quoted(join_continuations(command))
    matched = None
    for pattern, label in DESTRUCTIVE:
        if pattern.search(scannable):
            matched = label
            break
    if not matched:
        return

    cwd = payload.get("cwd") or ""
    where = targets(command, cwd)
    if where and all(worktree_kind(t, cwd) == "linked" for t in where):
        return  # every target is a linked worktree

    named = where[0] if where else "<cwd not reported; failing closed>"
    reason = (
        "Blocked destructive git ({0}) against the main checkout (target: {1}). "
        "This class of command is allowed only in a linked worktree. On this "
        "repository worktrees belong under C:\\dinah-scratch\\, never inside the "
        "checkout, because Dinah's workbench discovery climbs to the drive root "
        "and a nested worktree reaches the operator's live data. Create one with "
        "git -C <repo> worktree add --detach C:/dinah-scratch/<card>-<stage>/wt "
        "<ref>, then run this there (cd into it, or use git -C <worktree> ...). "
        "In the main checkout this is operator-only: ask the operator to run it, "
        "or the operator can disable this guard via /hooks (.claude/settings.json, "
        "andon-650)."
    ).format(matched, named)

    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }))


if __name__ == "__main__":
    main()
