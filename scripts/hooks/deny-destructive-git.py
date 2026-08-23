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

Finding the verbs is a regular-expression job, and this file has now
tried the alternative three times. Each attempt built a reader that split
the command into words and then walked the words looking for a
subcommand, and each one leaked somewhere its author had not imagined: a
whitespace-only tokeniser hid `echo a;git reset --hard`, and the
metacharacter-aware tokeniser that replaced it emitted `>` as a word of
its own, whereupon the walker underneath stopped on that `>`, called it
the subcommand, found it in no table, and cleared `git >/dev/null reset
--hard`. A fix at one layer opened a hole at the next.

The patterns below are the shape that has never leaked. Each one runs
from a `git` word, through a span of characters that cannot contain a
command separator, to the verb and to whichever flag decides the verb.
Robustness to punctuation is a property of the character class rather
than of a rule somebody remembered to write: a separator cannot hide an
invocation, because a separator ends the span, and a separator cannot
hide a flag, because `\b` sits between a flag and whatever is glued to
it. Nothing here tracks state, keeps a position, or infers a directory,
so nothing here reopens what OQ-9 deleted.

Three normalisations run before the patterns, and each one preserves the
length of the command so that a match's offsets still index the original
text. Line continuations are folded, because one invocation split over
two lines is still one invocation. Quoted spans are blanked, so a commit
message mentioning a command is an argument rather than a command.
Redirection operators are blanked, because `>` and `<` and the `&` of
`2>&1` are punctuation inside a single command rather than boundaries
between two, and leaving them in the character class was how the last
version cleared `git >/dev/null reset --hard`.

Permission is then read off the original text of the matched span. The
span reaches from the `git` word to the next character that could start
another command, so a `-C` in one invocation cannot vouch for another,
which is what `git -C <worktree> stash pop && git reset --hard` needs.

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
<worktree>` on every git command from every stage. A pattern reading a
span rather than a parse tree also refuses a little more than a parser
would: `git log --grep commit` carries the word `commit` outside quotes
and is refused. Refusing too much is the direction this guard is allowed
to be wrong in, and quoting the word is the fix.

Two things this guard does not cover and never did. A command that
changes directory and then runs something other than git is outside it,
because it reads git commands. And a git command assembled inside a
quoted string that another program then executes, as `eval "git reset
--hard"` does, is invisible to it, because blanking quoted spans is what
keeps a commit message from being read as a command and the two cannot
both be had.
"""

import json
import os
import re
import subprocess
import sys


GIT_TIMEOUT = 5

# Characters that can end one command and begin another. A span of the
# command that contains none of them is a single simple command, which is
# the unit permission is decided over. Redirection operators are blanked
# before the patterns run, so `>`, `<` and the `&` of `2>&1` never reach
# this class and cannot be mistaken for a boundary.
BOUNDARY_CHARACTERS = "|;&`(){}\n"

# The span between a `git` word and the verb it is running. Written once
# and spliced into every pattern, because a rule that spells its own
# character class is a rule that can spell it differently.
GAP = "[^" + re.escape(BOUNDARY_CHARACTERS) + "]*"

BOUNDARY = re.compile("[" + re.escape(BOUNDARY_CHARACTERS) + "]")

GIT = r"\bgit\b"

# A bundled short flag carrying a particular letter, as `-fdx` carries
# `f`. The trailing `\b` is what makes `-fdx;` and `-fdx` the same flag.
def short(letter):
    return r"\s-[a-z]*" + letter + r"[a-z]*\b"


def rule(*parts):
    return re.compile("".join(parts), re.IGNORECASE)


# A colon refspec, as in `git push origin :topic` or
# `git push origin HEAD:refs/heads/x`. A remote spelled as a URL carries a
# colon too, so `://` and a `user@host` prefix are both excluded.
COLON_REFSPEC = r"\s(?!-)[^\s:@" + re.escape(BOUNDARY_CHARACTERS) + r"]*:(?!//)"

DENIED = [
    (rule(GIT, GAP, r"\breset\b", GAP, r"(?:--hard|--merge|--keep)\b"),
     "git reset --hard/--merge/--keep"),
    (rule(GIT, GAP, r"\bclean\b", GAP, "(?:", short("f"), "|", short("d"), "|",
          short("x"), r"|\s--force\b)"),
     "git clean -f/-d/-x"),
    (rule(GIT, GAP, r"\bcheckout\b", GAP, r"(?:\s--(?![\w.=-])|", short("f"),
          r"|\s--force\b)"),
     "git checkout -- / -f"),
    (rule(GIT, GAP, r"\brestore\b(?!", GAP, r"--staged\b(?!", GAP, r"--worktree\b))"),
     "git restore (working tree)"),
    (rule(GIT, GAP, r"\bswitch\b", GAP, "(?:", short("f"),
          r"|\s--force\b|\s--discard-changes\b)"),
     "git switch -f/--force/--discard-changes"),
    (rule(GIT, GAP, r"\bpush\b", GAP, r"(?:\s--force(?!-with-lease)\b|", short("f"), ")"),
     "git push --force"),
    (rule(GIT, GAP, r"\bpush\b", GAP, r"--force-with-lease", GAP, r"\b(?:main|master)\b"),
     "git push --force-with-lease to main/master"),
    (rule(GIT, GAP, r"\bpush\b", GAP, r"(?:\s--delete\b|", short("d"), "|",
          COLON_REFSPEC, ")"),
     "git push --delete / colon refspec"),
    (rule(GIT, GAP, r"\bstash\b(?!\s+(?:list|show)\b)"),
     "git stash (mutating)"),
    (rule(GIT, GAP, r"\bcommit\b"), "git commit"),
    (rule(GIT, GAP, r"\bmerge\b(?!-)"), "git merge"),
    (rule(GIT, GAP, r"\brebase\b(?!-)"), "git rebase"),
    (rule(GIT, GAP, r"\bcherry-pick\b"), "git cherry-pick"),
    (rule(GIT, GAP, r"\brevert\b"), "git revert"),
    (rule(GIT, GAP, r"\bam\b"), "git am"),
    (rule(GIT, GAP, r"\bapply\b(?!", GAP, r"--(?:check|stat)\b)"), "git apply"),
    (rule(GIT, GAP, r"\brm\b(?!", GAP, r"--cached\b)"), "git rm"),
    (rule(GIT, GAP, r"\bmv\b"), "git mv"),
    (rule(GIT, GAP, r"\bbisect\b(?!\s+(?:log|view)\b)"), "git bisect"),
    (rule(GIT, GAP, r"\bpull\b(?!", GAP, r"--ff-only\b)"), "git pull (may merge)"),
    (rule(GIT, GAP, r"\bworktree\b", GAP, r"\bremove\b"), "git worktree remove"),
    (rule(GIT, GAP, r"\bbranch\b", GAP, r"(?:\s--delete\b|", short("d"), ")"),
     "git branch -d/-D"),
    (rule(GIT, GAP, r"\btag\b", GAP, r"(?:\s--delete\b|", short("d"), ")"),
     "git tag -d"),
]

# One real invocation split over two lines. Folded first so the flag on
# the second line still counts, and folded to spaces so the fold costs no
# characters and every offset below still indexes the original command.
CONTINUATION = re.compile(r"\\[ \t]*\r?\n")

# A quoted span. Blanked so prose cannot be read as a command. An
# unterminated quotation mark matches nothing here, which leaves the rest
# of the command in place and errs toward a refusal.
QUOTED = re.compile("\"[^\"]*\"|'[^']*'")

# A redirection operator, with the file descriptor in front of it and the
# `&` and descriptor of `2>&1` behind it. Blanked because none of it
# separates two commands, and because leaving `>` inside a word is how
# `git >/dev/null reset --hard` cleared the last version of this guard.
REDIRECTION = re.compile(r"\d*(?:>>|>&|>|<<<|<<|<&|<)\d*-?")

# The `-C` that grants permission, and it grants it only where git reads
# it: on the invocation itself, directly after the `git` word. A `-C`
# anywhere else vouches for nothing, which refuses rather than clears, so
# `git -c core.pager=cat -C <worktree> commit` is refused although git
# would honour the directory. Refusing a spelling nobody on this board
# writes is the safe direction, and the alternative is a walk over the
# words ahead of the verb, which is the shape that has leaked three times.
#
# The case fold covers the `git` word and stops there. `-c` and `-C` are
# different options, `-c` sets configuration and `-C` names a directory,
# and folding the case of the flag was itself a fail-open: this harness's
# first run caught `git -c core.pager=cat reset --hard` being cleared by
# a `-C` that was never written.
GIT_C = re.compile(r"(?i:\bgit\b)\s+-C\s*(\"[^\"]*\"|'[^']*'|\S+)")

# Every `-C` in the span, because git applies more than one cumulatively
# and the last one wins. All of them have to qualify.
ANY_C = re.compile(r"(?:^|\s)-C\s*(\"[^\"]*\"|'[^']*'|\S+)")

# Options naming a target the guard does not follow.
UNFOLLOWED = re.compile(r"(?:^|\s)--(?:git-dir|work-tree)\b", re.IGNORECASE)


def blank(text, pattern):
    """Replace every match of `pattern` with spaces of the same length.

    Length is preserved so that an offset into the normalised text is an
    offset into the original command. That is what lets the patterns read
    a command with its quoted spans removed while permission is read off
    the command as the agent wrote it.
    """
    return pattern.sub(lambda match: " " * len(match.group(0)), text)


def normalise(command):
    """The command as the patterns read it, character for character."""
    folded = blank(command, CONTINUATION)
    unquoted = blank(folded, QUOTED)
    return blank(unquoted, REDIRECTION)


def unquote(token):
    """Strip one matched surrounding pair of quotation marks."""
    if len(token) >= 2 and token[0] == token[-1] and token[0] in "\"'":
        return token[1:-1]
    return token


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


def fault(segment):
    """Why this invocation did not earn its permission, or None when it did.

    `segment` is the original text of one simple command, starting at the
    `git` word the pattern matched.
    """
    unfollowed = UNFOLLOWED.search(segment)
    if unfollowed:
        return "%s names a target this guard does not follow" % unfollowed.group(0).strip()
    anchored = GIT_C.search(segment)
    if not anchored:
        return "the invocation carries no -C"
    for match in ANY_C.finditer(segment):
        directory = unquote(match.group(1))
        if not os.path.isabs(directory):
            return "-C %s is relative, so it names no directory on its own" % directory
        kind = worktree_kind(directory)
        if kind == "main":
            return "-C %s is the main checkout" % directory
        if kind != "linked":
            return "-C %s is not a git worktree" % directory
    return None


def offender(command):
    """The first invocation the guard will not clear, or None when all are clear.

    Returns `(label, fault)`, where the label names the rule and the
    fault says why the invocation did not earn its permission.
    """
    scannable = normalise(command)
    for pattern, label in DENIED:
        for match in pattern.finditer(scannable):
            boundary = BOUNDARY.search(scannable, match.end())
            end = boundary.start() if boundary else len(scannable)
            why = fault(command[match.start():end])
            if why:
                return label, why
    return None


def decide(command):
    """The guard's verdict on a command, as `(label, fault)` or None.

    Split out from `main` so that a harness comparing this guard with a
    real shell reads the guard itself rather than a copy of its early
    bail.
    """
    if "git" not in command.lower():
        return None
    return offender(command)


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return  # unparseable payload: stay out of the way

    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command") or ""

    refusal = decide(command)
    if refusal is None:
        return
    matched, why = refusal

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
    ).format(matched, why)

    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }))


if __name__ == "__main__":
    main()
