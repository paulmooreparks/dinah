#!/usr/bin/env python3
r"""PreToolUse guard: deny repository-mutating git commands against the main checkout.

Reads the Claude Code hook payload on stdin. Denies when a git invocation
in the command carries a mutating verb AND the directory that invocation
will run in is the operator's own checkout. Fail-closed: a directory this
hook cannot classify counts as the main checkout.

Two questions decide every verdict, and each is answered in one place.

Which directory will the command run in? The shell's working directory
does not survive from one tool call to the next, so `cd <worktree> && git
...` is the ordinary way an agent works, and judging it by the session's
directory would refuse correct work. The guard reads a `cd` in the
command it is judging, an explicit `git -C`, and the payload's `cwd`, in
that order of authority per invocation. It runs as a PreToolUse hook,
before the tool call it is judging, so a `cd` inside that command cannot
yet be reflected in `cwd` whatever the harness does afterwards.

Is that directory the operator's checkout? `git rev-parse --git-dir
--git-common-dir` is the documented discriminator. The two answers differ
in a linked worktree, where the per-worktree git dir sits under
`<common>/worktrees/<name>`, and match in the main worktree. Nothing here
depends on where a worktree is created, so the guard stays correct if the
convention moves.

The deny set is chosen rather than inherited. A guard that refuses too
much stops every agent from doing ordinary work in the main checkout, and
one that refuses too little leaves the hole it was written to fill. What
is refused in the main checkout: reset --hard/--merge/--keep, clean with
-f/-d/-x, checkout -- / -f, restore of the working tree, push --force,
push --force-with-lease at main or master, stash other than list and
show, commit, merge, rebase, cherry-pick, revert, am, apply without
--check or --stat, rm without --cached, mv, bisect other than log and
view, switch with -f/--force/--discard-changes, pull without --ff-only,
worktree remove, branch -d/-D, tag -d, and push with --delete or a colon
refspec. The ref deletions are refused because refs are shared across
every worktree of the repository, so deleting one from the main checkout
reaches into whatever another agent is standing on.

What stays allowed in the main checkout: worktree add, list, and prune;
plain checkout and switch of a branch; fetch; push of a branch; branch
and tag used to create or list; stash list and show; and every read.
`worktree prune` clears the administrative record for a directory that is
already gone, skips a locked worktree, and destroys no commits, and it is
the only way to finish the cleanup the board's safety document requires.

The board's own stages run several of the refused verbs. They are
unaffected because each of those stages works in its own worktree, where
the guard classifies the target as linked and allows the command.
"""

import json
import os
import re
import shlex
import subprocess
import sys


GIT_TIMEOUT = 5

CONTINUATION = re.compile(r"\\[ \t]*\r?\n")

# `<<WORD`, `<<-WORD`, `<<"WORD"` and `<<'WORD'`. The body that follows is
# data rather than command text, so the segmenter drops it.
HEREDOC = re.compile(r"<<-?[ \t]*(?:\"([^\"]*)\"|'([^']*)'|([A-Za-z_][A-Za-z0-9_]*))")

# A word carrying `git` at all, used only to decide whether a segment that
# will not tokenise is worth refusing.
GIT_WORD = re.compile(r"\bgit\b")

# A separator across which a `cd` reaches a later invocation. The others
# (`||`, `|`, `&`) either run their git without the `cd` having succeeded
# or run it in a different process.
CARRYING = ("&&", ";", "\n")

# A quotation mark opens a span only at the start of a word. That is the
# model `shlex` uses in non-POSIX mode, where `git commit -m it's`
# tokenises with the apostrophe inside the word, and the segmenter has to
# agree with the tokeniser or the two disagree about where a command ends.
WORD_START = " \t\r\n(=&|;<>"

UNRESOLVED = "\x00unresolved"

# A `cd` target the shell expands before `cd` sees it. The guard cannot
# know what it becomes, so it resolves to nothing and fails closed.
EXPANDABLE = re.compile(r"[$`*?\[]|^~")


def join_continuations(command):
    r"""Fold shell line-continuations back onto one line.

    A newline separates commands exactly as `;` does, so a backslash
    continuation is the one case where a single invocation spans lines.
    Rejoining it first keeps `git reset \` followed by `--hard` readable
    as `git reset --hard`.
    """
    return CONTINUATION.sub(" ", command)


def word_initial(text, index):
    return index == 0 or text[index - 1] in WORD_START


def unquote(token):
    """Strip one matched surrounding pair of quotation marks.

    Non-POSIX tokenising leaves the marks attached to the token, and a
    path is wanted rather than its spelling.
    """
    if len(token) >= 2 and token[0] == token[-1] and token[0] in "\"'":
        return token[1:-1]
    return token


def skip_heredoc_bodies(text, index, delimiters):
    """Advance past the bodies of the heredocs opened on the line just ended."""
    for delimiter in delimiters:
        while index < len(text):
            end = text.find("\n", index)
            line = text[index:end if end != -1 else len(text)]
            index = len(text) if end == -1 else end + 1
            if line.strip() == delimiter:
                break
    return index


def split_top_level(text):
    """Split a command into `[(separator_before, segment)]`.

    A separator inside quotation marks or inside parentheses is not top
    level, and a heredoc body never reaches the output at all. The
    separator each segment carries is the one that preceded it, which is
    what decides whether a `cd` reaches it.
    """
    parts = []
    separator = ""
    buffer = []
    depth = 0
    pending = []
    index = 0
    length = len(text)

    def flush(next_separator):
        parts.append((separator, "".join(buffer)))
        del buffer[:]
        return next_separator

    while index < length:
        char = text[index]
        if char in "\"'" and word_initial(text, index):
            close = text.find(char, index + 1)
            if close == -1:
                buffer.append(text[index:])
                index = length
                break
            buffer.append(text[index:close + 1])
            index = close + 1
            continue
        if char == "(":
            depth += 1
            buffer.append(char)
            index += 1
            continue
        if char == ")":
            depth = max(0, depth - 1)
            buffer.append(char)
            index += 1
            continue
        if char == "<" and text.startswith("<<", index):
            opening = HEREDOC.match(text, index)
            if opening:
                pending.append(opening.group(1) or opening.group(2) or opening.group(3) or "")
                buffer.append(opening.group(0))
                index = opening.end()
                continue
        if char == "\n":
            index += 1
            if pending:
                index = skip_heredoc_bodies(text, index, pending)
                pending = []
            if depth == 0:
                separator = flush("\n")
            else:
                buffer.append("\n")
            continue
        if depth == 0:
            if text.startswith("&&", index):
                separator = flush("&&")
                index += 2
                continue
            if text.startswith("||", index):
                separator = flush("||")
                index += 2
                continue
            if char == ";":
                separator = flush(";")
                index += 1
                continue
            if char == "|":
                separator = flush("|")
                index += 1
                continue
            # `2>&1` and `&>log` spell a redirection rather than a
            # separator, and splitting there would lose a `cd` that
            # legitimately reaches the invocation after it.
            if char == "&" and text[index - 1:index] != ">" and text[index + 1:index + 2] != ">":
                separator = flush("&")
                index += 1
                continue
        buffer.append(char)
        index += 1

    parts.append((separator, "".join(buffer)))
    return parts


def split_group(segment):
    """Split `(inner) rest` into its interior and whatever follows it."""
    depth = 0
    index = 0
    length = len(segment)
    while index < length:
        char = segment[index]
        if char in "\"'" and word_initial(segment, index):
            close = segment.find(char, index + 1)
            if close == -1:
                break
            index = close + 1
            continue
        if char == "(":
            depth += 1
        elif char == ")":
            depth -= 1
            if depth == 0:
                return segment[1:index], segment[index + 1:]
        index += 1
    return segment[1:], ""


def cd_target(tokens):
    """The directory a `cd` moves to, or UNRESOLVED when it cannot be known."""
    arguments = [token for token in tokens[1:] if not token.startswith("-")]
    if not arguments:
        return UNRESOLVED
    target = unquote(arguments[0])
    if not target or EXPANDABLE.search(target):
        return UNRESOLVED
    return target


def git_slices(tokens):
    """Each git invocation in a segment, as its own token list."""
    starts = [position for position, token in enumerate(tokens)
              if os.path.basename(unquote(token)).lower() in ("git", "git.exe")]
    slices = []
    for order, start in enumerate(starts):
        end = starts[order + 1] if order + 1 < len(starts) else len(tokens)
        slices.append(tokens[start:end])
    return slices


TAKES_VALUE = ("-c", "--namespace", "--work-tree", "--git-dir", "--exec-path", "--super-prefix")


def read_invocation(invocation):
    """The `-C` path, the subcommand, and the subcommand's arguments."""
    directory = None
    index = 1
    while index < len(invocation):
        token = invocation[index]
        if token == "-C" and index + 1 < len(invocation):
            directory = unquote(invocation[index + 1])
            index += 2
            continue
        if token.startswith("-C") and len(token) > 2:
            directory = unquote(token[2:])
            index += 1
            continue
        if token in TAKES_VALUE:
            index += 2
            continue
        if token.startswith("-"):
            index += 1
            continue
        break
    if index >= len(invocation):
        return directory, None, []
    return directory, unquote(invocation[index]), [unquote(token) for token in invocation[index + 1:]]


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


def scan(text, inherited, findings):
    """Collect `(label, directory)` for every denied invocation in one sequence.

    `inherited` is the directory this sequence starts in: None for the
    payload's own working directory, a path, or UNRESOLVED. A `cd`
    rebinds it for the segments that follow across a carrying separator,
    and a parenthesised segment gets its own copy that does not escape.
    """
    current = inherited
    for separator, segment in split_top_level(text):
        if separator and separator not in CARRYING:
            current = inherited
        stripped = segment.strip()
        if not stripped:
            continue
        if stripped.startswith("("):
            inner, rest = split_group(stripped)
            scan(inner, current, findings)
            if rest.strip():
                scan(rest, current, findings)
            continue
        try:
            tokens = shlex.split(stripped, posix=False)
        except ValueError:
            # An unterminated quotation mark leaves the segment
            # unreadable, and a hook that cannot read a command cannot
            # clear it.
            if GIT_WORD.search(stripped):
                findings.append(("unreadable command", UNRESOLVED))
            continue
        if not tokens:
            continue
        if unquote(tokens[0]) == "cd":
            current = cd_target(tokens)
            continue
        for invocation in git_slices(tokens):
            directory, subcommand, arguments = read_invocation(invocation)
            label = denied(subcommand, arguments)
            if label:
                findings.append((label, directory if directory is not None else current))


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


def offender(findings, cwd):
    """The first finding the guard will not clear, or None when all are clear.

    An invocation whose directory cannot be resolved is refused, which is
    the answer the guard has always given a payload carrying no working
    directory.
    """
    for label, directory in findings:
        if directory == UNRESOLVED:
            return label, "<no directory resolved; failing closed>"
        if directory is None:
            if not cwd:
                return label, "<cwd not reported; failing closed>"
            directory = cwd
        if worktree_kind(directory, cwd) != "linked":
            return label, directory
    return None


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return  # unparseable payload: stay out of the way

    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command") or ""
    if "git" not in command:
        return

    findings = []
    scan(join_continuations(command), None, findings)
    if not findings:
        return

    cwd = payload.get("cwd") or ""
    refusal = offender(findings, cwd)
    if refusal is None:
        return
    matched, named = refusal

    reason = (
        "Blocked repository-mutating git ({0}) against the main checkout (target: {1}). "
        "This class of command is allowed only in a linked worktree. Note that the "
        "shell's working directory does not persist between calls: it resets to the "
        "session's primary directory, which is the operator's checkout, so every "
        "command has to carry its own directory, as either cd <worktree> && git ... or "
        "git -C <worktree> ... . On this repository worktrees belong under "
        "C:\\dinah-scratch\\, never inside the checkout, because Dinah's workbench "
        "discovery climbs to the drive root and a nested worktree reaches the "
        "operator's live data. Create one with git -C <repo> worktree add --detach "
        "C:/dinah-scratch/<card>-<stage>/wt <ref>. In the main checkout this is "
        "operator-only: ask the operator to run it, or the operator can disable this "
        "guard via /hooks (.claude/settings.json)."
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
