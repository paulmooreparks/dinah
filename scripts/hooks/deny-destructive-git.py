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
form the guard reads. `-c core.worktree=` and `-c core.bare=` are the
configuration spellings of the same setting and are refused with them,
because a principle that turns on which of two documented spellings an
agent reached for is not a principle. The environment spelling is a
third, and it is outside this guard rather than covered by it:
`GIT_WORK_TREE=<path> git -C <worktree> reset --hard` carries nothing
in its command text that a reader could refuse. A relative `-C` is
refused for a related reason: it names a directory only in combination
with a working directory, and the working directory is exactly what the
guard has stopped consulting.

Finding the verbs is a regular-expression job, and this file has now
tried the alternative three times. Each attempt built a reader that split
the command into words and then walked the words looking for a
subcommand, and each one leaked somewhere its author had not imagined: a
whitespace-only tokeniser hid `echo a;git reset --hard`, and the
metacharacter-aware tokeniser that replaced it emitted `>` as a word of
its own, whereupon the walker underneath stopped on that `>`, called it
the subcommand, found it in no table, and cleared `git >/dev/null reset
--hard`. A fix at one layer opened a hole at the next.

A verb is matched only where it stands as a whole shell word, and the
two lookarounds that say so are the narrowest thing this file could do
about a defect that refused read-only commands. `\b` is a regular
expression's idea of a word rather than a shell's, and a hyphen ends
one, so `merge` was found inside the path `dinah-249-merge`, `am` inside
`card-am`, and `rm` inside `rm-fixture`. The guard refused
`git -C C:/dinah-scratch/dinah-249-merge/wt status --short` and told the
agent it had run a merge. It also forbade the command this file's own
refusal text prescribes, because a worktree built for the merge stage is
`<card>-merge/wt` and the command that creates one carries `-C` at the
main checkout by necessity.

That fix is not the reader this file abandoned three times, and the
difference is worth stating rather than assuming. Nothing added here
walks words, skips options, keeps a position, or decides which word is
the subcommand. Every rule still fires on its verb wherever the verb
stands in the span, and all that changed is which characters count as
joining a word to its neighbour.

Whether that is safe turns on one question: is a character the
lookaround refuses a character git will really receive? The first
version of this argument answered yes and was wrong. It claimed that
every refused character, left in the text, hands git a longer token than
the verb, so a rule declining to match could never be a rule missing a
command. But four of the seven refused characters are also shell
punctuation. `-`, `=`, `+` and `/` are the operators of a parameter
expansion, so each of them can stand in the command text immediately
beside a literal verb and be deleted from it before git is started.
`git ${x-reset} --hard` runs a hard reset, and the whole-word rules
decline to read the verb it runs. Every verb in the deny set can be
written that way, and so can the flag that condemns one, as
`git reset ${x---hard}`.

The real question is therefore which shell constructs can take a
character away from beside a verb, and there are three. Quote removal is
the first and line continuation is the second. Both are normalised
above, and both normalisations put a space where the removed characters
stood. A space is not a refused character and it joins nothing, so after
those two the guard sees a verb standing more alone than the shell does
and can only refuse more than it should. Where quoting destroys the verb
itself, as `git 'reset' --hard` does, this guard and the one on the
trunk are blind together, which the last section of this header has
always said.

Expansion is the third, and it is the one that removes a character and
puts nothing in its place. Enumerating its spellings is the trap this
file has fallen into before, so nothing here tries to understand an
expansion. The rules withdraw the relaxation wherever one could be doing
the deleting. A span carrying `$`, a backtick, a brace or a parenthesis
is read a second time by the same rules written the way the trunk wrote
them, with a plain `\b` on each verb and no left anchor on any flag, and
either reading is enough to refuse. One function builds both tables, so
the two readings are one rule set read two ways rather than two copies
that drift apart, which is the failure the single `GIT_WORD` pattern
below was written after.

What that buys is a comparison rather than a claim about shells. On a
span carrying expansion syntax this guard refuses everything the guard
on the trunk refuses, and on a span carrying none it refuses everything
the trunk refuses except the path-shaped false positives this change
exists to remove. An expansion spelled in a way nobody here has thought
of cannot get past that, because what turns the second reading on is the
presence of the punctuation rather than the shape of it. The cost falls
where expansion syntax and a verb-named path share a span and the `-C`
does not earn the command its permission on its own. A read through a
merge-stage worktree that has already been removed is allowed as
`git -C C:/dinah-scratch/dinah-250-merge/wt log --oneline -1` and
refused again once `$(date)` stands in the same span, because the
second reading finds the `merge` in the path that the first reading
declined to read.

Tilde expansion, filename expansion and history expansion are outside
that trigger, and each is left out for a reason rather than forgotten. A
tilde prefix becomes a home directory and a glob becomes a filename, so
both hand git a longer word instead of a bare verb, and history
expansion is off in the shell a hook's commands run in. None of the
three can delete a character and leave nothing behind, which is the only
thing the trigger is watching for.

A backslash is deliberately not among the refused characters, and
leaving it out costs a refusal on Windows. A backslash in front of a
word is how a shell quotes the letter after it, so reading it as a
joining character would clear `git \reset --hard`. The price is that a
backslash-spelled path whose segment is exactly a deny-set verb, as
`C:\dinah-scratch\merge\wt`, still reads as carrying that verb, and
every path this board writes uses forward slashes.

The patterns below are the shape that has never leaked. Each one runs
from a `git` word, through a span of characters that cannot contain a
command separator, to the verb and to whichever flag decides the verb.
Robustness to punctuation is a property of the character class rather
than of a rule somebody remembered to write: a separator cannot hide an
invocation, because a separator ends the span, and a separator cannot
hide a flag, because a flag is delimited by what a word cannot contain
rather than by whitespace. Requiring whitespace in front of a flag was
itself a fail-open, in eight of the rules at once, and `git {clean,-fdx}`
is what a shell does with a brace expansion. Nothing here tracks state,
keeps a position, or infers a directory, so nothing here reopens what
OQ-9 deleted.

Four normalisations run before the patterns, and each one preserves the
length of the command so that a match's offsets still index the original
text. Line continuations are folded, because one invocation split over
two lines is still one invocation. Quoted spans are blanked, so a commit
message mentioning a command is an argument rather than a command.
Redirection operators are blanked, because `>` and `<` and the `&` of
`2>&1` are punctuation inside a single command rather than boundaries
between two, and leaving them in the character class was how the last
version cleared `git >/dev/null reset --hard`. Boundary characters
standing inside a substitution are blanked for the same reason the
redirection operators are: a `;` between two commands inside `$( )`
separates those two and does not end the command the substitution is a
word of, and leaving it there ended the span early and cleared `git
$(echo;) reset --hard`.

Blanking a quoted span is where the second and third of those meet, and
the two kinds of quotation mark do not earn the same treatment. A shell
runs nothing inside single quotation marks, so a single-quoted span is
data all the way through. It runs a substitution inside double quotation
marks exactly as it runs one outside them, so a double-quoted span is
data except for its substitutions, which stay visible to the rules.
Blanking both kinds alike hid `git -C <a linked worktree> commit -m
"$(git reset --hard)"` from the guard while bash ran the reset, and
`git commit -m "$(...)"` is a spelling agents reach for.

Permission is then decided over the matched span, which reaches from the
`git` word to the next character that could start another command, so a
`-C` in one invocation cannot vouch for another and `git -C <worktree>
stash pop && git reset --hard` is refused.

Detection and permission read the same text, and an earlier version of
this file did not. It matched patterns against the normalised command and
then looked for the `-C` in the original, so the two halves disagreed
about which characters were data: `git reset --hard "git -C <a real
worktree> x"` was cleared by a `-C` git never sees, and that string is
the exact idiom every agent definition on this board and this guard's own
refusal text tell an agent to write. The `-C` token is now found in the
normalised text, where a quoted span is blank, and only its argument is
read from the original, so a `-C` inside quotation marks grants nothing
while `git -C "C:/dinah scratch/wt" stash pop` still names its worktree.

A span holding more than one `git` word is refused outright. The span
crosses a parenthesis, a brace and a backtick, because a shell keeps all
of those inside one command, and the price of crossing them is that two
invocations can share a span. Which of them a `-C` belongs to is then a
question the guard cannot answer, and it refuses rather than guesses.

Counting those words and finding them are one pattern, and they were two
for exactly one cycle. The finder matched `git` with a word boundary on
each side, which is what `./git`, `/usr/bin/git` and `git.exe` carry; the
counter's own expression disqualified all three. A span therefore read as
one invocation while the shell ran two, and the `-C` on the one that
carried it cleared the one that did not. Seven strings leaked that way,
`git -C <a linked worktree> stash list $(./git reset --hard)` among them,
while the same string spelled `$(git reset --hard)` was correctly
refused. Keeping two patterns in step is what failed, so there is one
pattern, and it is the finder's. Breadth belongs to the finder because a
wider finder detects more invocations, and a counter no narrower than the
finder cannot read a span as simpler than the finder read it. The cost is
that `.git/config` or `git-lfs` standing beside a deny-set verb now
refuses that span, which is a refusal rather than a leak.

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
<worktree>` on every git command from every stage. A `-C` whose path is
held in a shell variable is refused as well, so `git -C $WT commit` and
`git -C "$WT" commit` do not pass; the guard reads text, and `$WT` names
a directory only once a shell has expanded it, which is why the refusal
calls the path relative. A pattern reading a span rather than a parse
tree also refuses a little more than a parser would: `git log --grep
commit` carries the word `commit` outside quotes and is refused, and
quoting the word clears it. The one-git-word-per-span rule has its own
cost, and it falls on a command substitution inside a deny-set
invocation, so `git -C <worktree> commit -m "wip" $(git -C <worktree>
rev-parse HEAD)` and `git -C <worktree> cherry-pick $(git -C <worktree>
rev-parse HEAD)` are refused although both of their invocations name the
same worktree. The same rule refuses an unquoted `-C` whose path carries
a git component, as `git -C /home/x/git/repo commit`, because those
letters between two separators are what a git word looks like; quoting
the path clears it, and the board's own worktrees carry no such
component. Double-quoting the substitution does not clear those and
is not meant to, because a shell runs a substitution inside double
quotation marks; running the inner command first and passing its output
is what clears them. Refusing too much is the direction this guard is
allowed to be wrong in.

Two things this guard does not cover and never did. A command that
changes directory and then runs something other than git is outside it,
because it reads git commands.

And quoting hides a command from it, which is a wider hole than the one
example everybody reaches for. `eval "git reset --hard"` is the famous
spelling, but quoting any part of the git word or the verb does the same
thing for the same reason: `git 'reset' --hard`, `git re""set --hard`
and `"git" reset --hard` all run the verb and all pass, because blanking
a quoted span destroys the word inside it. Blanking is what keeps a
commit message from being read as a command, and a guard cannot both
ignore quoted text and read it. The trunk's guard has always had this
hole too, and closing it is not a matter of handling another spelling.
A substitution written inside double quotation marks is not part of this
hole and never was: the shell runs it, so the guard reads it, which is
what the normalisation above is for.
"""

import json
import os
import re
import subprocess
import sys


GIT_TIMEOUT = 5

# Characters that can end one command and begin another. A span of the
# command that contains none of them is at most one simple command, which
# is the unit permission is decided over. Redirection operators are
# blanked before the patterns run, so `>`, `<` and the `&` of `2>&1` never
# reach this class and cannot be mistaken for a boundary.
#
# These four and no more, which is the set the guard on the trunk has
# always used. A wider set was tried and it was a fail-open: a shell keeps
# a parenthesis, a brace and a backtick inside a single command, so a span
# that stops at one of them stops before the verb, and `git ${OPTS} reset
# --hard`, `git $(echo --no-pager) reset --hard`, the backtick form,
# `git${IFS}reset --hard` and `git {reset,--hard}` all ran with the guard
# reporting nothing to refuse. The file had already reasoned this out for
# redirection one line down and then stopped short of expansion and
# substitution.
BOUNDARY_CHARACTERS = "|;&\n"

# The span between a `git` word and the verb it is running. Written once
# and spliced into every pattern, because a rule that spells its own
# character class is a rule that can spell it differently.
GAP = "[^" + re.escape(BOUNDARY_CHARACTERS) + "]*"

BOUNDARY = re.compile("[" + re.escape(BOUNDARY_CHARACTERS) + "]")

GIT = r"\bgit\b"

# Where a word can begin. Written as a refusal of the characters that
# continue a word rather than as a list of the characters that separate
# them, because the separating characters are not knowable: whitespace is
# the ordinary one, but brace expansion writes a word boundary as `{` or
# `,`, and `git {clean,-fdx}` runs `git clean -fdx` with no whitespace in
# it at all. Requiring whitespace was a fail-open in eight of the rules
# below at once, and the differential harness produced 300 strings for
# it. The `@` is refused so that `git@host:x/y` stays a remote rather
# than a word beginning at `host`.
WORD_START = r"(?<![\w.=/~@+-])"

# Where a word ends, spelled as the mirror of WORD_START so that the two
# refuse the same characters. A git subcommand is a whole shell word, and
# these two together are all that says so. The header records why that is
# a different question from the one the retired parser asked, and what
# the backslash's absence from the set costs.
WORD_END = r"(?![\w.=/~@+-])"


# A bundled short flag carrying a particular letter, as `-fdx` carries
# `f`. The trailing `\b` is what makes `-fdx;` and `-fdx` the same flag.
def short(letter):
    return WORD_START + r"-[a-z]*" + letter + r"[a-z]*\b"


# A verb where it stands as a whole shell word rather than inside one.
def verb(word):
    return WORD_START + word + WORD_END


# A long flag where it begins a word. The left side is the side a path
# takes away, as `--hard` inside `seed--hard.txt`, and it is the side
# `short` has always guarded. The right side keeps `\b`: WORD_END there
# would stop the `--force` rule reading `--force-if-includes`, and a rule
# that stops refusing a forced push is not a tidying-up.
def flag(*names):
    return "(?:" + WORD_START + "(?:" + "|".join(names) + r")\b)"


# The same three spelled without the left anchor, for the second reading
# of a span that carries expansion syntax. A verb keeps a plain `\b` and
# a flag carries no anchor at all, so nothing in this set can be talked
# out of a match by a character standing beside the match. That is the
# point of it: where the shell's punctuation cannot be trusted, the
# guard trusts none of it.
#
# Two of these are stricter than the trunk rather than equal to it.
# `anywhere_short` and `ANYWHERE_COLON_REFSPEC` drop a left anchor that
# the trunk's own `short` and `COLON_REFSPEC` carry, because
# `git clean ${x--fdx}` and `git push origin ${x-:topic}` are real
# commands the trunk misses for the reason the header gives, and a
# second reading written to be no stricter than the trunk would have
# carried both holes forward.
def anywhere_short(letter):
    return r"-[a-z]*" + letter + r"[a-z]*\b"


def anywhere_verb(word):
    return r"\b" + word + r"\b"


def anywhere_flag(*names):
    return "(?:" + "|".join(names) + r")\b"


def rule(*parts):
    return re.compile("".join(parts), re.IGNORECASE)


# A colon refspec, as in `git push origin :topic` or
# `git push origin HEAD:refs/heads/x`. A remote spelled as a URL carries a
# colon too, so `://` and a `user@host` prefix are both excluded.
COLON_REFSPEC = WORD_START + r"(?!-)[^\s:@" + re.escape(BOUNDARY_CHARACTERS) + r"]*:(?!//)"

ANYWHERE_COLON_REFSPEC = r"(?!-)[^\s:@" + re.escape(BOUNDARY_CHARACTERS) + r"]*:(?!//)"

# The bare `--` that ends git's own options, which is what makes
# `git checkout -- .` a discard of the working tree rather than a branch
# switch.
SEPARATOR = WORD_START + r"--(?![\w.=-])"

ANYWHERE_SEPARATOR = r"--(?![\w.=-])"

# `merge-base` and `rebase-todo` are not the verbs, and the second
# reading has to say so itself because a plain `\b` ends a word at a
# hyphen. WORD_END already refuses the hyphen in the first reading,
# where this lookahead is redundant and harmless.
NOT_HYPHENATED = r"(?!-)"


def denied(verb, flag, short, colon, separator):
    r"""The deny set, spelled with one set of word boundaries.

    Called twice, and the two calls are the whole of the difference
    between the guard's two readings of a span. Writing the table out
    twice instead would be two tables to keep in step, and keeping two
    patterns in step is what produced the counting defect recorded
    further down this file.

    The operand of the `--force-with-lease` rule takes no boundary from
    the caller, because a plain `\b` is right there under either reading.
    A ref is written `refs/heads/main` and `HEAD:refs/heads/main` as
    readily as `main`, so requiring `main` to be a whole shell word would
    clear a forced push at the trunk.
    """
    return [
        (rule(GIT, GAP, verb("reset"), GAP, flag("--hard", "--merge", "--keep")),
         "git reset --hard/--merge/--keep"),
        (rule(GIT, GAP, verb("clean"), GAP, "(?:", short("f"), "|", short("d"), "|",
              short("x"), "|", flag("--force"), ")"),
         "git clean -f/-d/-x"),
        (rule(GIT, GAP, verb("checkout"), GAP, "(?:", separator, "|", short("f"),
              "|", flag("--force"), ")"),
         "git checkout -- / -f"),
        (rule(GIT, GAP, verb("restore"), "(?!", GAP, flag("--staged"), "(?!", GAP,
              flag("--worktree"), "))"),
         "git restore (working tree)"),
        (rule(GIT, GAP, verb("switch"), GAP, "(?:", short("f"),
              "|", flag("--force", "--discard-changes"), ")"),
         "git switch -f/--force/--discard-changes"),
        (rule(GIT, GAP, verb("push"), GAP, "(?:", flag("--force(?!-with-lease)"),
              "|", short("f"), ")"),
         "git push --force"),
        (rule(GIT, GAP, verb("push"), GAP, flag("--force-with-lease"), GAP,
              r"\b(?:main|master)\b"),
         "git push --force-with-lease to main/master"),
        (rule(GIT, GAP, verb("push"), GAP, "(?:", flag("--delete"), "|", short("d"), "|",
              colon, ")"),
         "git push --delete / colon refspec"),
        (rule(GIT, GAP, verb("stash"), r"(?!\s+(?:list|show)\b)"),
         "git stash (mutating)"),
        (rule(GIT, GAP, verb("commit")), "git commit"),
        (rule(GIT, GAP, verb("merge"), NOT_HYPHENATED), "git merge"),
        (rule(GIT, GAP, verb("rebase"), NOT_HYPHENATED), "git rebase"),
        (rule(GIT, GAP, verb("cherry-pick")), "git cherry-pick"),
        (rule(GIT, GAP, verb("revert")), "git revert"),
        (rule(GIT, GAP, verb("am")), "git am"),
        (rule(GIT, GAP, verb("apply"), "(?!", GAP, flag("--check", "--stat"), ")"),
         "git apply"),
        (rule(GIT, GAP, verb("rm"), "(?!", GAP, flag("--cached"), ")"), "git rm"),
        (rule(GIT, GAP, verb("mv")), "git mv"),
        (rule(GIT, GAP, verb("bisect"), r"(?!\s+(?:log|view)\b)"), "git bisect"),
        (rule(GIT, GAP, verb("pull"), "(?!", GAP, flag("--ff-only"), ")"),
         "git pull (may merge)"),
        (rule(GIT, GAP, verb("worktree"), GAP, verb("remove")), "git worktree remove"),
        (rule(GIT, GAP, verb("branch"), GAP, "(?:", flag("--delete"), "|", short("d"), ")"),
         "git branch -d/-D"),
        (rule(GIT, GAP, verb("tag"), GAP, "(?:", flag("--delete"), "|", short("d"), ")"),
         "git tag -d"),
    ]


# The first reading, where a verb has to stand as a whole shell word.
# This is the reading that stopped `merge` being found inside
# `dinah-249-merge`.
DENIED = denied(verb, flag, short, COLON_REFSPEC, SEPARATOR)

# The second reading, for a span whose punctuation the shell may be
# about to eat. `offender` runs these only where EXPANSION matches the
# span, so the path-shaped false positives the first reading removes
# stay removed everywhere else.
DENIED_ANYWHERE = denied(anywhere_verb, anywhere_flag, anywhere_short,
                         ANYWHERE_COLON_REFSPEC, ANYWHERE_SEPARATOR)

# The punctuation that introduces or delimits an expansion, a
# substitution, or a grouping a shell can expand. Its presence anywhere
# in a span is what turns the second reading on, because these are the
# constructs that delete a character from beside a verb and put nothing
# in its place. What is read is the presence of the punctuation rather
# than its shape, so an expansion spelled in a way nobody here has
# enumerated still turns the second reading on. This runs over the
# normalised text, where a quoted span is blank, because a `$` inside
# quotation marks expands nothing that reaches git.
EXPANSION = re.compile(r"[$`{}()]")

# One real invocation split over two lines. Folded first so the flag on
# the second line still counts, and folded to spaces so the fold costs no
# characters and every offset below still indexes the original command.
CONTINUATION = re.compile(r"\\[ \t]*\r?\n")

# A quoted span. Blanked so prose cannot be read as a command. An
# unterminated quotation mark matches nothing here, which leaves the rest
# of the command in place and errs toward a refusal.
QUOTED = re.compile("\"[^\"]*\"|'[^']*'")

# A command substitution. The shell runs one of these wherever it stands,
# including inside double quotation marks, where everything else is data.
# That distinction is the whole reason this pattern exists: blanking a
# double-quoted span entirely hid `git -C <a linked worktree> commit -m
# "$(git reset --hard)"` from the guard while bash ran the reset, and
# `git commit -m "$(...)"` is a spelling agents write. Single quotation
# marks are a different matter and stay fully blanked, because a shell
# runs nothing inside them.
#
# Non-greedy, so a substitution the pattern cannot delimit exactly is cut
# short rather than missed. Cutting it short leaves the text ahead of the
# cut visible to the rules, which is the direction that refuses.
SUBSTITUTION = re.compile(r"\$\([\s\S]*?\)|`[^`]*`")

# A boundary character standing inside a substitution is blanked with
# `BOUNDARY`, the same pattern the span reader uses. A `;` between two
# commands inside `$( )` separates those two commands and does not end
# the command the substitution is a word of, so leaving it in the text
# ends the span early and hides whatever follows: `git $(echo;) reset
# --hard` runs a bare reset and had no verb in it as far as the guard
# could see. This is the reasoning the redirection blanking below already
# carries, applied to the other punctuation one command can hold.

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
# and folding the case of the flag was itself a fail-open: the
# differential harness's first run caught `git -c core.pager=cat reset
# --hard` being cleared by a `-C` that was never written.
#
# These two match the flag only. Their offsets are used against the
# normalised text, where a quoted span is blank, so a `-C` an agent wrote
# inside a commit message is not a `-C` git will ever see. The value is
# then read out of the original text at the offset the flag ended, which
# is what keeps `git -C "C:/dinah scratch/wt" stash pop` working: the
# token has to stand outside quotation marks, and its argument may be
# inside them.
GIT_C = re.compile(r"(?i:\bgit\b)\s+-C")
ANY_C = re.compile(r"(?:^|\s)-C")

# The argument a `-C` names, read from the original text.
C_VALUE = re.compile(r"\s*(\"[^\"]*\"|'[^']*'|\S+)")

# A `git` word, counted rather than located, and it is `GIT` itself
# rather than a second pattern written to resemble it. The counter used
# to be its own expression, narrower than the finder by a lookaround that
# disqualified `./git`, `/usr/bin/git` and `git.exe`, and that difference
# was a fail-open: the finder saw two invocations where the counter saw
# one, so `git -C <a linked worktree> stash list $(./git reset --hard)`
# was cleared by a `-C` belonging to the other command. Written plainly
# as `$(git reset --hard)` the same string was refused, which is what
# said the rule was right and the counter was wrong.
#
# The two cannot simply be kept in step, because keeping two patterns in
# step is the shape that produced this defect and several before it. They
# are one pattern, and the breadth is the finder's because the finder's
# is the safe direction: a wide finder detects more invocations, and a
# counter no narrower than the finder cannot report a span as simpler
# than the finder read it. Erring the other way costs a refusal, and
# erring this way costs the guard.
#
# The price is paid where the letters appear without being the command.
# `.git/config`, `git-lfs`, `--git-dir`, `git@host` and `git://host` now
# count as git words, so a span already carrying a deny-set verb is
# refused when one of them stands beside it. None of them is refused on
# its own, because a span is only ever counted after a rule has matched
# in it, and refusing too much is the direction this guard is allowed to
# be wrong in.
#
# The instance of that worth knowing about is a `-C` path with a git
# component in it, as `git -C /home/x/git/repo commit` or a worktree
# under a directory named for this very tool. Quoting the path clears it,
# because a quoted span is blank by the time anything is counted, and the
# board's own worktrees live under `C:\dinah-scratch\<card>-<stage>\wt`
# and carry no such component. The suite's own temporary directory did
# carry one, which is how this was found rather than reported.
GIT_WORD = re.compile(GIT, re.IGNORECASE)

# Options naming a target the guard does not follow. `--git-dir` and
# `--work-tree` are the spellings on the command line; `-c core.worktree=`
# and `-c core.bare=` are the configuration spellings of the same
# setting, documented in git-config(1), and an invocation carrying either
# has said no more about where it runs than one carrying the option.
#
# The environment spelling, `GIT_WORK_TREE=<path> git -C <worktree>
# reset --hard`, is outside what a guard reading command text can
# resolve, and it is named here rather than handled.
UNFOLLOWED = re.compile(
    WORD_START + r"(?:--(?:git-dir|work-tree)\b|-c\s*core\.(?:worktree|bare)\b)",
    re.IGNORECASE,
)


def blank(text, pattern):
    """Replace every match of `pattern` with spaces of the same length.

    Length is preserved so that an offset into the normalised text is an
    offset into the original command. That is what lets the patterns read
    a command with its quoted spans removed while permission is read off
    the command as the agent wrote it.
    """
    return pattern.sub(lambda match: " " * len(match.group(0)), text)


def blank_around_substitutions(span):
    """`span` blanked, except for the substitutions a shell still runs in it.

    Used on a double-quoted span, where everything is data except a
    command substitution. Same length out as in, like every other
    normalisation here.
    """
    kept = [" "] * len(span)
    for inner in SUBSTITUTION.finditer(span):
        kept[inner.start():inner.end()] = span[inner.start():inner.end()]
    return "".join(kept)


def unquote_spans(text):
    """`text` with its quoted spans blanked, by the rule each kind earns.

    A single-quoted span is data all the way through. A double-quoted
    span is data except for the substitutions inside it, which the shell
    runs exactly as it runs them outside.
    """
    def replace(match):
        span = match.group(0)
        if span.startswith("'"):
            return " " * len(span)
        return blank_around_substitutions(span)

    return QUOTED.sub(replace, text)


def open_substitutions(text):
    """`text` with the boundary characters inside substitutions blanked.

    A separator inside `$( )` separates the commands in there and does
    not end the command outside, so it must not end the span the guard
    reads.
    """
    def replace(match):
        return blank(match.group(0), BOUNDARY)

    return SUBSTITUTION.sub(replace, text)


def normalise(command):
    """The command as the patterns read it, character for character."""
    folded = blank(command, CONTINUATION)
    unquoted = unquote_spans(folded)
    redirected = blank(unquoted, REDIRECTION)
    return open_substitutions(redirected)


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


def named_directory(raw, position):
    """The directory a `-C` names, read from the original text at `position`.

    `position` is where the flag ended in the normalised text, and the
    two texts share offsets by construction, so it is where the flag
    ended in the original too. Returns None when the flag names nothing.
    """
    value = C_VALUE.match(raw, position)
    return unquote(value.group(1)) if value else None


def fault(raw, scan):
    """Why this invocation did not earn its permission, or None when it did.

    `raw` is the original text of one span of the command, starting at
    the `git` word the pattern matched, and `scan` is the same span of
    the normalised text. Detection and permission read the same
    characters as data and the same characters as command, which is what
    stops a `-C` written inside a quoted argument from vouching for an
    invocation that carries none.
    """
    unfollowed = UNFOLLOWED.search(scan)
    if unfollowed:
        return "%s names a target this guard does not follow" % unfollowed.group(0).strip()
    # More than one `git` word in a span the guard reads as one command
    # means it cannot say which invocation a `-C` belongs to, and a
    # question the guard cannot answer is refused rather than guessed.
    # The span crosses a parenthesis, a brace and a backtick, so this is
    # what keeps `git -C <worktree> log $(git reset --hard)` refused.
    # `GIT_WORD` is `GIT`, so what counts an invocation here is what
    # found one above; a counter narrower than the finder read
    # `$(./git reset --hard)` as no invocation at all.
    if len(GIT_WORD.findall(scan)) > 1:
        return "the span carries more than one git word, so no -C in it is readable"
    anchored = GIT_C.search(scan)
    if not anchored:
        return "the invocation carries no -C"
    for match in ANY_C.finditer(scan):
        directory = named_directory(raw, match.end())
        if directory is None:
            return "-C names no directory"
        if not os.path.isabs(directory):
            return "-C %s is relative, so it names no directory on its own" % directory
        kind = worktree_kind(directory)
        if kind == "main":
            return "-C %s is the main checkout" % directory
        if kind != "linked":
            return "-C %s is not a git worktree" % directory
    return None


# How much of the matched text a refusal quotes, and how a run of
# whitespace in it is written. A match reaches from the `git` word to the
# flag that condemns it, so it can carry a whole `-C` path and a message;
# the cap keeps the refusal readable, and the collapse keeps it on one
# line inside a JSON string.
EVIDENCE_LIMIT = 160
WHITESPACE = re.compile(r"\s+")


def evidence(matched):
    """The text a rule matched, on one line and short enough to read."""
    collapsed = WHITESPACE.sub(" ", matched).strip()
    if len(collapsed) > EVIDENCE_LIMIT:
        collapsed = collapsed[:EVIDENCE_LIMIT - 3] + "..."
    return collapsed


def offender(command):
    """The first invocation the guard will not clear, or None when all are clear.

    Returns `(label, fault, matched)`. The label names the rule, the
    fault says why the invocation did not earn its permission, and
    `matched` is the text the rule actually matched, taken from the
    command as the agent wrote it.

    A refusal quotes that text rather than naming the label alone,
    because the label is what the guard read and not necessarily what the
    command runs. A rule reads a span rather than a parse tree, so
    `git log --grep commit` matches the commit rule; a message saying
    only "git commit" tells the reader something the command does not
    contain, and a reader who cannot see what matched cannot tell a
    correct refusal from a defective one.

    Every span is read by the whole-word rules, and a span carrying
    expansion syntax is read again by the rules that anchor nothing.
    Either reading is enough to refuse, which is what makes the second
    one safe to add: it can only take a span away from the allowed side.
    The header says why the second reading is needed, and the short
    version is that four of the characters the whole-word rules refuse
    are also parameter-expansion operators, so a verb can sit beside one
    in the text and reach git without it.
    """
    scannable = normalise(command)
    for rules, needs_expansion in ((DENIED, False), (DENIED_ANYWHERE, True)):
        for pattern, label in rules:
            for match in pattern.finditer(scannable):
                boundary = BOUNDARY.search(scannable, match.end())
                end = boundary.start() if boundary else len(scannable)
                span = scannable[match.start():end]
                if needs_expansion and not EXPANSION.search(span):
                    continue
                why = fault(command[match.start():end], span)
                if why:
                    return label, why, command[match.start():match.end()]
    return None


def decide(command):
    """The guard's verdict on a command, as `(label, fault, matched)` or None.

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
    matched, why, text = refusal

    reason = (
        "Blocked repository-mutating git. The text \"{2}\" stands outside quotation "
        "marks in this command, and this guard reads it as {0}; the invocation was "
        "refused because {1}. A command running this class of git verb has to name "
        "its worktree on the invocation itself, as "
        "git -C <worktree> ... , and nothing else grants it permission. The shell's "
        "working directory does not persist between calls: it resets to the session's "
        "primary directory, which is the operator's checkout, so a command that does "
        "not say where it runs runs there. On this repository worktrees belong under "
        "C:\\dinah-scratch\\, never inside the checkout, because Dinah's workbench "
        "discovery climbs to the drive root and a nested worktree reaches the "
        "operator's live data. Create one with git -C <repo> worktree add --detach "
        "C:/dinah-scratch/<card>-<stage>/wt <ref>. If the quoted text above is an "
        "argument rather than the subcommand git will run, quoting that argument "
        "keeps it out of this scan. If this command really does belong "
        "in the main checkout, it is the operator's to run, or the operator can "
        "disable this guard via /hooks (.claude/settings.json)."
    ).format(matched, why, evidence(text))

    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason,
        }
    }))


if __name__ == "__main__":
    main()
