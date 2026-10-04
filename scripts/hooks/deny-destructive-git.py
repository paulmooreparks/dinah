#!/usr/bin/env python3
"""PreToolUse guard: no destructive git command runs in the main checkout.

The main checkout is the operator's working copy and holds the live
workbench, which git ignores. A destructive git verb run there can throw
away uncommitted work or, through `git clean -x`, delete the workbench.
Every other directory is fair game, the scratch worktrees above all.

For each git invocation carrying a destructive verb, the guard works out
where it runs and refuses it only when that place is the main checkout or
inside it. Where it runs is, in order: the `-C` the invocation names; else
the last directory change earlier in the same command; else the session's
working directory from the hook payload. A directory change the guard
cannot read, such as one built from a variable, counts as unknown and is
refused. A `git worktree remove` is judged by the worktree it names rather
than by where it runs.

What this does not cover, stated so nobody reads more into it: the
GIT_DIR and GIT_WORK_TREE environment variables, a script file that runs
git, and git invoked by another program. `--git-dir`, `--work-tree` and
`core.worktree` on a destructive invocation are refused outright.
"""

import json
import os
import re
import subprocess
import sys
from pathlib import Path

# A verb counts only as a whole shell word, so `merge` inside the path
# `dinah-249-merge` and `rm` inside `rm-fixture` are not read as verbs.
W = r"(?<![\w./\\-])"
E = r"(?![\w./\\-])"

DESTRUCTIVE = [
    (W + r"reset" + E + r".*--(?:hard|merge|keep)" + E, "git reset --hard/--merge/--keep"),
    (W + r"clean" + E + r".*(?:\s-[a-zA-Z]*[fdxX]|--force)", "git clean"),
    (W + r"checkout" + E + r".*(?:\s--(?:\s|$)|\s-f" + E + r"|--force)", "git checkout -- / -f"),
    (W + r"restore" + E + r"(?!.*--staged(?!.*--worktree))", "git restore"),
    (W + r"switch" + E + r".*(?:\s-f" + E + r"|--force|--discard-changes)", "git switch --force"),
    (W + r"stash" + E + r"(?!\s+(?:list|show)" + E + r")", "git stash"),
    (W + r"rm" + E + r"(?!.*--cached)", "git rm"),
    (W + r"worktree\s+remove" + E, "git worktree remove"),
]

SEGMENT_SPLIT = re.compile(r"&&|\|\||[;|\n]")
GIT = re.compile(r"(?:^|[\s\"'&(])git(?:\.exe)?\s+(.*)$", re.IGNORECASE)
DIR_CHANGE = re.compile(
    r"^\s*(?:cd|pushd|chdir|set-location|sl|push-location)\s+(?:-path\s+)?(.+?)\s*$",
    re.IGNORECASE)
C_FLAG = re.compile(r"(?:^|\s)-C\s+(\"[^\"]*\"|'[^']*'|\S+)")
UNFOLLOWED = re.compile(r"--git-dir|--work-tree|core\.worktree|core\.bare")
WORKTREE_TARGET = re.compile(r"\bworktree\s+remove\s+(?:--force\s+|-f\s+)*(\"[^\"]*\"|'[^']*'|\S+)")
UNKNOWN = None


def normal(raw, base):
    """An absolute, comparable spelling of a path, or UNKNOWN."""
    text = raw.strip().strip("\"'")
    if not text or re.search(r"[$`%]|^~", text):
        return UNKNOWN
    msys = re.match(r"^/([a-zA-Z])(/|$)", text)
    if msys:
        text = msys.group(1) + ":/" + text[len(msys.group(0)):]
    path = Path(text)
    if not path.is_absolute():
        if base is UNKNOWN:
            return UNKNOWN
        path = Path(base) / path
    return os.path.normcase(os.path.normpath(str(path)))


def inside(path, main):
    return path == main or path.startswith(main.rstrip("\\/") + os.sep)


def decide(command, cwd, main):
    """None to allow, or a sentence saying what was refused and why."""
    if "git" not in command.lower():
        return None
    main = normal(main, None)
    here = normal(cwd, None) if cwd else UNKNOWN
    for segment in SEGMENT_SPLIT.split(command):
        change = DIR_CHANGE.match(segment)
        if change:
            here = normal(change.group(1), here)
            continue
        git = GIT.search(segment)
        if not git:
            continue
        rest = git.group(1)
        label = next((name for pattern, name in DESTRUCTIVE if re.search(pattern, rest)), None)
        if label is None:
            continue
        if UNFOLLOWED.search(rest):
            return f"{label} names its repository with an option this guard does not follow"
        flag = C_FLAG.search(rest)
        where = normal(flag.group(1), here) if flag else here
        if label == "git worktree remove":
            target = WORKTREE_TARGET.search(rest)
            gone = normal(target.group(1), where) if target else UNKNOWN
            if gone is UNKNOWN or gone == main:
                return f"{label} does not name a worktree this guard can read"
            continue
        if where is UNKNOWN:
            return f"{label} runs in a directory this guard cannot work out"
        if inside(where, main):
            return f"{label} would run in the main checkout"
    return None


def main_checkout():
    here = Path(__file__).resolve().parent
    out = subprocess.run(["git", "-C", str(here), "rev-parse", "--path-format=absolute",
                          "--git-common-dir"], capture_output=True, text=True, timeout=5)
    return str(Path(out.stdout.strip()).parent) if out.returncode == 0 else str(here.parents[1])


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return
    command = (payload.get("tool_input") or {}).get("command") or ""
    why = decide(command, payload.get("cwd") or os.getcwd(), main_checkout())
    if why is None:
        return
    reason = (
        f"Refused: {why}. The main checkout is the operator's working copy and holds "
        "the live workbench. Run destructive git in a worktree under C:/dinah-scratch, "
        "naming it with git -C <worktree> or a cd to it earlier in the same command. "
        "If this really belongs in the main checkout, it is the operator's to run."
    )
    print(json.dumps({"hookSpecificOutput": {
        "hookEventName": "PreToolUse",
        "permissionDecision": "deny",
        "permissionDecisionReason": reason,
    }}))


if __name__ == "__main__":
    main()
