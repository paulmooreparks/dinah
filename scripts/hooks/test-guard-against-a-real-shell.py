#!/usr/bin/env python3
r"""Differential harness: the destructive-git guard against a real shell.

Run it from anywhere: `python scripts/hooks/test-guard-against-a-real-shell.py`.
It exits non-zero when the guard cleared a command that then reached git
with a verb the guard is supposed to refuse.

WHY THIS EXISTS. Three implementations of this guard shipped a fail-open,
every one found by a person reading the code and none by a suite that had
grown to 567 hand-written cases. The cause was not capability. Each
author built a generator, and each generator could only produce the
shapes its author had already imagined, so the suite inherited the
author's blind spot and reported green over the hole. What found all
three was a reviewer using bash as an oracle: a fake `git` first on PATH,
a real shell, and a comparison between what the guard decided and what
the command actually did. Three reviewers improvised that setup by hand
and threw it away again, so this file keeps it.

The question it asks is not "did the author think of this spelling". It
is "does the guard agree with the shell", and the shell is the authority
because the shell is what runs the command.

HOW IT WORKS. A directory holding one executable named `git` goes first
on PATH. That `git` writes its own arguments into a file of its own under
a recording directory, and exits.
Each generated string is then run twice: once through the guard, which
answers allow or deny, and once through bash with the stub in front,
which answers with the argument vectors git was actually handed. A
failure is one thing and one thing only: the guard said allow, and the
stub received an argument vector whose verb is in the deny set and which
carries no `-C` naming a linked worktree. That is a command the guard let
through and the shell then ran.

The reverse disagreement, where the guard refuses a string the shell
would have run harmlessly, is counted and printed but is not a failure.
Refusing too much is the direction this guard is allowed to be wrong in,
and a pattern reading a span rather than a parse tree is expected to be
wrong in it sometimes.

THE SECOND ORACLE IS THE GUARD ALREADY DEPLOYED. Every generated string
is also put to the version of this hook on `origin/main`, and a string
the trunk refuses and this branch allows fails the run. Two consecutive
cycles of this card shipped exactly that: a hole in a place where the
operator already had protection, invisible to a harness that only ever
asked whether the branch agreed with a shell. A branch may refuse more
than the trunk and it may refuse differently, but it may not quietly
refuse less. The trunk's own file is read out of git rather than
reimplemented, and it is driven through the interface a hook has, which
is a JSON payload on stdin and a decision on stdout, so nothing here has
an opinion about how the trunk reaches its verdict.

The oracle that reads the recorded arguments is deliberately not the
guard's own code. It walks an argument vector, which is a list the shell
already split, so it needs no shell grammar and shares no reasoning with
the thing it is checking. Built out of the guard it would inherit the
guard's mistakes and report agreement on every one of them.

A GENERATOR IS ONLY AS GOOD AS ITS SHAPES, and every cycle of this card
has found one missing. The shape found missing most recently was two git
invocations inside a single span. The crossing that pairs a qualifying
invocation with a bare one always put a separator between them, so the
guard's span ended in the gap and each invocation was judged on its own,
and not one string in the file reached the rule that refuses a span
holding two invocations. Arming said so out loud and nobody read it:
disabling that rule reddened two cases in the fast suite and nothing at
all here. The strings are generated now, with the second invocation
spelled four ways, because the guard's counter once excluded three of
them and cleared the reset that followed.

A RELAXATION IS DECLARED OR IT IS A REGRESSION. The rule above stops a
branch quietly refusing less than the trunk, and a card whose whole
purpose is to refuse less would fail every run under it. So a string this
branch means to allow is generated with a name that says so, and the
harness then holds it to both halves of the claim: the trunk has to
refuse it, or the declaration is stale and has stopped describing
anything, and this branch has to allow it, or the fix it was written for
has been reverted. Every declared string still goes to the shell like any
other, so a relaxation that lets a deny-set verb through is a fail-open
and is reported as one.

SAFETY. Every generated string is git and nothing else, wrapped in
punctuation. `git` is the stub, so no repository is touched, and the
shell runs with its working directory in a temporary sandbox. The one
real repository the run builds is its own, under a temporary directory,
and it exists only so that a `-C` can name a genuine linked worktree.
"""

import concurrent.futures
import io
import itertools
import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import importlib.util


HERE = os.path.dirname(os.path.abspath(__file__))
HOOK = os.path.join(HERE, "deny-destructive-git.py")

# Where the already-deployed guard is read from. A ref rather than a file,
# because the question is what the operator is protected by today.
TRUNK_REF = "origin/main"
TRUNK_PATH = "scripts/hooks/deny-destructive-git.py"

# How many shells run at once. The work is a process start rather than
# arithmetic, so more workers than cores still pays, and the count comes
# off the machine rather than off the one this was written on. The floor
# keeps the run finite where `cpu_count` answers None.
WORKERS = max(4, (os.cpu_count() or 4) * 2)

# How long to wait after a shell returns before reading the recording
# directory, so that an invocation the string backgrounded is not missed.
SETTLE = 0.05

# The stub. It writes the arguments it was handed into a file of its own
# under the recording directory, one argument per line, and then
# succeeds. A file per invocation rather than a shared log with markers
# in it, because a generated string may background one invocation beside
# another and two shells appending to one file interleave their lines.
# That is not hypothetical: the shared-log spelling reported a single
# invocation reading "tag -C -d <path> tag v1.0 -d v1.0", which no shell
# ever ran, and the harness then called its own scrambling a fail-open.
#
# Written for /bin/sh rather than bash so that it costs nothing to start,
# and it must never do anything but record: this file's whole claim is
# that the shell rather than a model decides what a string runs.
STUB = """#!/bin/sh
out=$(mktemp "$GIT_STUB_DIR/rec.XXXXXXXX")
for argument in "$@"; do
  printf '%s\\n' "$argument"
done > "$out"
exit 0
"""


def load_module(path, name):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load_guard():
    """The guard module itself, imported rather than copied.

    The harness reads the real `decide`, so a change to the guard's early
    bail or to its patterns is a change to what this harness tests.
    """
    return load_module(HOOK, "deny_destructive_git")


def fetch_trunk_guard(root):
    """The deployed guard's source, written out of git into `root`.

    Returns the path, or None when the ref is not available, which is the
    case on a clone with no remote and on a machine that has not fetched.
    A missing trunk costs the comparison rather than the run, and the
    report says so out loud instead of reporting a green nobody earned.
    """
    try:
        blob = subprocess.run(
            ["git", "show", "%s:%s" % (TRUNK_REF, TRUNK_PATH)],
            cwd=HERE, capture_output=True, text=True, timeout=60,
        )
    except Exception:
        return None
    if blob.returncode != 0 or not blob.stdout.strip():
        return None
    path = os.path.join(root, "trunk-deny-destructive-git.py")
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(blob.stdout)
    return path


class Trunk:
    """The deployed guard, asked the same question through its own interface.

    A hook's interface is a JSON payload on stdin and a decision on
    stdout, and that is what this drives. Reaching inside for a function
    would bind the comparison to the trunk's internal shape, and the
    trunk is a fixed point this branch is measured against rather than a
    library it may assume things about.

    The stream swap is process-wide, so the calls are serialised. What
    they serialise is regex work over a few dozen characters once the
    worktree answers are cached, and the shells the other threads are
    running write to pipes rather than to this process's stdout.
    """

    def __init__(self, module, cwd):
        self.module = module
        self.cwd = cwd
        self.lock = threading.Lock()
        self.errors = 0
        kinds = {}
        original = module.worktree_kind

        # One argument, because that is what `worktree_kind` takes. This
        # wrapper asked for two, so every trunk verdict on a command
        # carrying a `-C` raised a TypeError, `refuses` swallowed it, and
        # the string was recorded as one the deployed guard allows. The
        # second oracle was answering "no refusal" for precisely the
        # commands it exists to compare, which is why an unanswerable
        # trunk is now fatal to the run rather than a number in the
        # report.
        def cached(target):
            if target not in kinds:
                kinds[target] = original(target)
            return kinds[target]

        module.worktree_kind = cached

    def refuses(self, command):
        """True when the deployed guard denies this command.

        A command the trunk cannot read at all counts as no verdict
        rather than as a refusal, because counting it as a refusal would
        make every such string a failure of this branch for something the
        trunk never did. The count is kept, and the run fails on it: a
        trunk that answers nothing is not a comparison, and the report
        used to carry that number where nobody read it.
        """
        payload = json.dumps({"tool_input": {"command": command}, "cwd": self.cwd})
        with self.lock:
            stdin, stdout = sys.stdin, sys.stdout
            sys.stdin = io.StringIO(payload)
            sys.stdout = io.StringIO()
            try:
                self.module.main()
                written = sys.stdout.getvalue()
            except Exception:
                self.errors += 1
                return False
            finally:
                sys.stdin, sys.stdout = stdin, stdout
        return '"deny"' in written


def git(*args, cwd=None):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True)


def build_repo(root):
    """A main checkout with one commit and two linked worktrees beside it.

    The second worktree's path carries a space, because a path with a
    space in it has to be quoted, and a quoted `-C` argument is the
    legitimate case that any fix to the quoted-argument hole must not
    break. Without it the harness cannot tell a guard that reads a quoted
    path correctly from one that has stopped reading quoted paths at all.
    """
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
    # A worktree named for a card at the merge stage, which is the shape
    # this board's own convention produces and the shape whose verb the
    # guard used to read as a subcommand.
    verbnamed = os.path.join(root, "scratch", "dinah-249-merge", "wt")
    git("worktree", "add", "--detach", "-q", verbnamed, "HEAD", cwd=main)
    return main, linked, spaced, verbnamed


# ---------------------------------------------------------------------------
# The oracle, which reads argument vectors and knows no shell grammar.
# ---------------------------------------------------------------------------

VALUED = ("-c", "--namespace", "--exec-path", "--super-prefix",
          "--git-dir", "--work-tree")


def read_argv(argv):
    """`(-C path or None, subcommand or None, arguments)` from a real argv."""
    directory = None
    index = 0
    while index < len(argv):
        token = argv[index]
        if token == "-C" and index + 1 < len(argv):
            directory = argv[index + 1]
            index += 2
            continue
        if token.startswith("-C") and len(token) > 2:
            directory = token[2:]
            index += 1
            continue
        if token in VALUED:
            index += 2
            continue
        if token.startswith("-"):
            index += 1
            continue
        break
    if index >= len(argv):
        return directory, None, []
    return directory, argv[index], argv[index + 1:]


def has(arguments, *names):
    return any(argument in names for argument in arguments)


def bundled(arguments, letters):
    for argument in arguments:
        if argument.startswith("-") and not argument.startswith("--") and len(argument) > 1:
            if any(letter in argument[1:] for letter in letters):
                return True
    return False


def colon_refspec(argument):
    if argument.startswith("-") or ":" not in argument:
        return False
    return "://" not in argument and "@" not in argument


def dangerous(subcommand, arguments):
    """The deny set, decided over an argument vector the shell produced."""
    if subcommand is None:
        return None
    if subcommand == "reset" and has(arguments, "--hard", "--merge", "--keep"):
        return "reset --hard/--merge/--keep"
    if subcommand == "clean" and (bundled(arguments, "fdxX") or has(arguments, "--force")):
        return "clean -f/-d/-x"
    if subcommand == "checkout" and ("--" in arguments or bundled(arguments, "f")
                                     or has(arguments, "--force")):
        return "checkout -- / -f"
    if subcommand == "restore" and not (has(arguments, "--staged")
                                        and not has(arguments, "--worktree")):
        return "restore (working tree)"
    if subcommand == "switch" and (bundled(arguments, "f")
                                   or has(arguments, "--force", "--discard-changes")):
        return "switch -f/--force/--discard-changes"
    if subcommand == "push":
        if has(arguments, "--force") or bundled(arguments, "f"):
            return "push --force"
        if has(arguments, "--delete") or bundled(arguments, "d"):
            return "push --delete"
        if any(colon_refspec(argument) for argument in arguments):
            return "push a colon refspec"
        return None
    if subcommand == "stash" and (not arguments or arguments[0] not in ("list", "show")):
        return "stash (mutating)"
    if subcommand == "commit":
        return "commit"
    if subcommand in ("merge", "rebase", "cherry-pick", "revert", "am"):
        return subcommand
    if subcommand == "apply" and not has(arguments, "--check", "--stat"):
        return "apply"
    if subcommand == "rm" and not has(arguments, "--cached"):
        return "rm"
    if subcommand == "mv":
        return "mv"
    if subcommand == "bisect" and (not arguments or arguments[0] not in ("log", "view")):
        return "bisect"
    if subcommand == "pull" and not has(arguments, "--ff-only"):
        return "pull (may merge)"
    if subcommand == "worktree" and arguments and arguments[0] == "remove":
        return "worktree remove"
    if subcommand == "branch" and (has(arguments, "--delete") or bundled(arguments, "dD")):
        return "branch -d/-D"
    if subcommand == "tag" and (has(arguments, "--delete") or bundled(arguments, "d")):
        return "tag -d"
    return None


# ---------------------------------------------------------------------------
# The generator.
# ---------------------------------------------------------------------------

# One command per deny-set rule, in the plainest spelling of it. Every
# shape below is applied to every one of these, so the crossing rather
# than the list is what covers the ground.
VERBS = [
    "git re" + "set --hard origin/main",
    "git clean -fdx",
    "git checkout -- .",
    "git checkout -f main",
    "git restore seed.txt",
    "git switch -f main",
    "git switch --discard-changes main",
    "git push --force origin topic",
    "git push origin --delete topic",
    "git push origin :topic",
    "git stash pop",
    "git stash",
    "git commit -m wip",
    "git merge origin/main",
    "git rebase origin/main",
    "git cherry-pick abc1234",
    "git revert abc1234",
    "git am patch.mbox",
    "git apply patch.diff",
    "git rm seed.txt",
    "git mv seed.txt other.txt",
    "git bisect start",
    "git pull origin main",
    "git worktree remove old",
    "git branch -D topic",
    "git tag -d v1.0",
]


def wrappings():
    """Punctuation and layout put around a whole command.

    Each entry is a format holding one `%s`. A shell runs every one of
    these exactly as it runs the bare command, so a guard that reads the
    bare command and not these is reading a different language.
    """
    return [
        ("bare", "%s"),
        ("semicolon in front", "echo a;%s"),
        ("&& in front", "true&&%s"),
        ("|| in front", "false||%s"),
        ("pipe in front", "echo a|%s"),
        ("ampersand in front", "echo a & %s"),
        ("newline in front", "echo a\n%s"),
        ("parenthesised", "(%s)"),
        ("brace group", "{ %s ; }"),
        ("brace group, no spaces", "{%s;}"),
        ("if/then/fi", "if true;then %s;fi"),
        ("for loop", "for i in 1;do %s;done"),
        ("while loop", "while false;do %s;done"),
        ("case arm", "case x in x)%s;;esac"),
        ("dollar-paren substitution", "echo $(%s)"),
        ("backtick substitution", "echo `%s`"),
        ("nested substitution", "echo $(echo $(%s))"),
        ("redirection glued behind", "%s>/dev/null"),
        ("append redirection glued behind", "%s>>/dev/null"),
        ("stderr redirection glued behind", "%s2>/dev/null"),
        ("stdin redirection glued behind", "%s</dev/null"),
        ("semicolon glued behind", "%s;"),
        ("pipe glued behind", "%s|cat"),
        ("double semicolon behind", "%s;;"),
        ("environment assignment in front", "FOO=bar %s"),
        ("leading whitespace", "   %s"),
        ("trailing newline", "%s\n"),
        ("after a heredoc", "cat <<EOF\ntext\nEOF\n%s"),
        ("after a here-string", "cat <<<word\n%s"),
        ("after a heredoc with a hyphenated marker", "cat <<E-OF\ntext\nE-OF\n%s"),
        ("after a comment line", "# a note\n%s"),
        ("subshell then the command", "(echo a) ; %s"),
        ("two deep in parentheses", "((%s))"),
        ("negated", "! %s"),
        ("timed", "time %s"),
    ]


def between_git_and_verb():
    """What can stand between `git` and its subcommand and still be one command.

    This is the position that has now leaked twice. In the fourth cycle a
    lexer split `>` out as a word of its own and the reader underneath
    called that `>` the subcommand, which cleared `git >/dev/null reset
    --hard`. In the fifth the span the guard read refused to cross a
    parenthesis, a brace or a backtick, so `git $(echo --no-pager) reset
    --hard` had no verb in it as far as the guard could see, and the
    trunk had been refusing that string all along.

    Each entry carries its own leading separator rather than relying on
    the caller to supply one, because `${IFS}` is a real spelling of the
    separator itself and `git${IFS}reset --hard` has no space in it at
    all. Every one of these is a real invocation of the verb that
    follows, so a guard reading any of them as anything else is wrong.
    """
    return [
        ("stdout redirection", " >/dev/null "),
        ("stderr redirection", " 2>/dev/null "),
        ("stderr onto stdout", " 2>&1 "),
        ("stdin redirection", " </dev/null "),
        ("append redirection", " >>/dev/null "),
        ("descriptor duplication", " 3>&1 "),
        ("line continuation", " \\\n  "),
        ("two continuations", " \\\n \\\n "),
        ("redirection then continuation", " >/dev/null \\\n "),
        ("global option", " --no-pager "),
        ("global option then redirection", " --no-pager >/dev/null "),
        ("configuration override", " -c core.pager=cat "),
        # The expansion and substitution family, which the generator went
        # without for three cycles and which is where the fifth cycle's
        # regression lived.
        ("an unset parameter expansion", " ${OPTS} "),
        ("a parameter expansion with a default", " ${OPTS:-} "),
        ("command substitution", " $(echo --no-pager) "),
        ("empty command substitution", " $(true) "),
        ("backtick substitution", " `echo --no-pager` "),
        ("IFS as the separator, no space anywhere", "${IFS}"),
        ("IFS after a global option", " --no-pager${IFS}"),
        ("substitution then redirection", " $(echo --no-pager) >/dev/null "),
        ("a single-element brace group", " {--no-pager} "),
        # A substitution carrying a boundary character. The separator
        # belongs to the commands inside the substitution and does not
        # end the command outside it, so a guard that reads it as a
        # boundary stops before the verb. Arming put these here: blanking
        # them was a fix nothing in this file could redden.
        ("a substitution carrying a semicolon", " $(echo;) "),
        ("a substitution carrying a pipe", " $(echo|cat) "),
    ]


def brace_expanded(command):
    """The command with its whole argument list written as a brace expansion.

    `git {reset,--hard}` runs `git reset --hard`, and the verb is inside
    a construct a guard reading spans has to cross. Bash leaves a
    single-element brace alone, so a command with one word after `git`
    has no brace form and is skipped rather than generated wrong.
    """
    words = command.split(" ")[1:]
    if len(words) < 2 or any("," in word for word in words):
        return None
    return "git {%s}" % ",".join(words)


def quoted_arguments(linked):
    """Layouts that hang a quoted argument off the command.

    The generator never quoted an argument anywhere, which is how the
    fifth cycle shipped a guard whose detector read the command with its
    quoted spans blanked while its permission check read the raw text. A
    `-C` written inside a quoted argument then vouched for an invocation
    that carried none. It is reachable rather than theoretical, because
    `git -C C:/dinah-scratch/...` is the exact string every agent
    definition, four columns and this guard's own refusal text tell an
    agent to write, so an agent quoting the board's advice in a commit
    message disarms the guard against itself.
    """
    idiom = "git -C %s x" % linked
    return [
        ("a double-quoted argument holding the board's own idiom", '%%s "%s"' % idiom),
        ("a single-quoted argument holding the board's own idiom", "%%s '%s'" % idiom),
        ("a quoted argument mentioning the idiom in prose",
         '%%s "see %s first"' % idiom),
        ("a quoted -C with no git word in front of it",
         '%%s "-C %s"' % linked),
        ("a quoted argument carrying nothing special", '%s "a note"'),
    ]


def glued_to_each_flag(command):
    """The command with a metacharacter glued to each of its flags."""
    shapes = []
    words = command.split(" ")
    for position, word in enumerate(words):
        if not word.startswith("-"):
            continue
        for label, glue in (("semicolon", ";"), ("redirection", ">/dev/null"),
                            ("pipe", "|cat")):
            spelled = list(words)
            spelled[position] = word + glue
            shapes.append(("%s glued to %s" % (label, word), " ".join(spelled)))
    return shapes


def qualified(command, path):
    """The same command with a `-C <path>` naming where it runs."""
    return 'git -C "%s" %s' % (path, command[len("git "):])


def second_invocation_spellings(stub):
    """Ways of writing a git invocation that are not the bare word `git`.

    The guard finds an invocation with a word-bounded `git`, which is
    what each of these carries, and for one cycle it counted invocations
    with a narrower expression that carried none of them. A span then
    read as one invocation while the shell ran two, and the `-C` on the
    one that had it cleared the one that did not. These four spellings
    are what tell the two apart, so a counter that narrows again is
    caught here rather than by a reader.

    On this platform `git.exe` and `git` name the same file, so the
    extension needs no second stub; the shell resolves it and the
    recording is the stub's.
    """
    return [
        ("plainly", "git"),
        ("with a relative path", "./git"),
        ("with an absolute path", stub.replace("\\", "/")),
        ("with an executable extension", "git.exe"),
    ]


def two_in_one_span(linked, stub):
    """Two invocations with no boundary character between them.

    The crossing lower down pairs a qualifying invocation with a bare one
    across a separator, so the guard's span ends between them and each is
    judged alone. This block is the case that crossing cannot reach: a
    substitution keeps both invocations inside one span, because the span
    deliberately crosses a parenthesis and a backtick, and the guard's
    answer there is to refuse a span it cannot attribute a `-C` to. Not
    one of the strings generated anywhere else in this file puts two
    invocations in one span, so nothing else here exercises that rule at
    all, and disabling the rule used to redden nothing.

    The substitution is written unquoted, in double quotation marks and
    in single ones, because the three are three different commands. A
    shell runs a substitution inside double quotation marks and the guard
    has to see it; a shell runs nothing inside single ones and the guard
    is right to see a string there.

    The last shape carries no qualifying invocation at all. `echo git -C
    <a linked worktree> $(git reset --hard)` puts the board's own idiom
    in the span as bare text, which is the sharpest statement of the
    defect: no real permission is needed to clear the span, only the
    letters of one.
    """
    shapes = []
    reader = 'git -C "%s" stash list' % linked
    writer = 'git -C "%s" commit -m wip' % linked
    for command, (how, word) in itertools.product(
            VERBS, second_invocation_spellings(stub)):
        inner = word + " " + command[len("git "):]
        for where, spelled in (
                ("substituted into a qualifying read", "%s $(%s)" % (reader, inner)),
                ("substituted into a qualifying read, double-quoted",
                 '%s "$(%s)"' % (reader, inner)),
                ("substituted into a qualifying read, single-quoted",
                 "%s '$(%s)'" % (reader, inner)),
                ("in backticks inside a qualifying read", "%s `%s`" % (reader, inner)),
                ("substituted into a qualifying deny-set invocation",
                 "%s $(%s)" % (writer, inner)),
                ("substituted into a qualifying deny-set invocation, double-quoted",
                 '%s "$(%s)"' % (writer, inner)),
                ("substituted beside the bare text of the idiom",
                 'echo git -C "%s" $(%s)' % (linked, inner))):
            shapes.append(("%s, second invocation spelled %s, %s"
                           % (command, how, where), spelled))
    return shapes


# The name a declared relaxation carries. A string generated under this
# prefix is one this branch intends to allow and the deployed guard
# refuses; anything else that the trunk refuses and this branch allows is
# a regression.
RELAXATION = "declared relaxation: "

# The deny-set verbs a path segment can trip on its own. The rules left
# out want a flag as well, or a second word, and a directory name
# supplies neither, so a path alone never reached them.
PATH_VERBS = ["restore", "stash", "commit", "merge", "rebase", "cherry-pick",
              "revert", "am", "apply", "rm", "mv", "bisect", "pull"]

# The two verbs the deployed guard already excluded a trailing hyphen
# after, so that `merge-base` and `rebase-todo` stayed out of its rules.
# A segment beginning with one of them was therefore never refused, and
# claiming it as a relaxation would claim something the trunk never did.
# The fast suite still holds both shapes, because there the question is
# what the guard does rather than what it stopped doing.
HYPHEN_EXCLUDED = ("merge", "rebase")


def slashed(path):
    """`path` written with forward slashes, which is how this board writes one.

    A backslash is how a shell quotes the letter after it rather than a
    separator, and the guard does not read one as ending a word, so a
    backslash-spelled fixture would ask a different question from the one
    these strings are for.
    """
    return path.replace("\\", "/")


def declares_relaxation(name):
    return name.startswith(RELAXATION)


def relaxations(checkout, verbnamed):
    """Strings this branch allows and the deployed guard refuses.

    Every verb in the deny set is also a word a directory can be named
    after, and this board names them that way, because a worktree belongs
    to a card at a stage and is written `<card>-<stage>/wt`. A regular
    expression ends a word at a hyphen where a shell does not, so the
    deployed guard reads `merge` out of `dinah-249-merge` and refuses a
    status, a log or a diff for a verb the command never ran. The worst
    of them is the last string here, which is the command the guard's own
    refusal text prescribes: a worktree is added from the repository it
    belongs to, so a creating command names the main checkout with its
    `-C` and cannot avoid doing so.

    The count is fixed and small, at three shapes for each of the verbs a
    path can trip plus two spelled-out cases, because every string here
    starts a shell.
    """
    scratch = slashed(os.path.dirname(os.path.dirname(verbnamed)))
    shapes = []
    for name in PATH_VERBS:
        for shape, segment in (("ending a segment", "card-%s" % name),
                               ("beginning a segment", "%s-fixture" % name),
                               ("standing as a whole segment", name)):
            if shape == "beginning a segment" and name in HYPHEN_EXCLUDED:
                continue
            shapes.append((RELAXATION + "%s %s of a -C path" % (name, shape),
                           "git -C %s/%s/wt rev-parse HEAD" % (scratch, segment)))
    shapes.append((RELAXATION + "a status read through a merge-stage path",
                   "git -C %s/dinah-250-merge/wt status --short" % scratch))
    shapes.append((RELAXATION + "the board's own merge-stage worktree is created",
                   "git -C %s worktree add --detach %s/dinah-293-merge/wt HEAD"
                   % (slashed(checkout), scratch)))
    return shapes


def strings(checkout, linked, spaced, verbnamed, stub):
    """Every generated string, as `(name, command)`.

    The crossing is deliberate and it is the point. Each deny-set verb is
    written in each layout, each verb is written again with punctuation
    between `git` and the subcommand, each of those is written again
    inside every layout, and every flag of every verb is written again
    with punctuation glued to it. Nobody chose which combinations were
    interesting, which is the property the three hand-written suites
    lacked.
    """
    generated = []

    for command, (shape, form) in itertools.product(VERBS, wrappings()):
        generated.append(("%s, %s" % (command, shape), form % command))

    for command, (where, insert) in itertools.product(VERBS, between_git_and_verb()):
        spelled = "git" + insert + command[len("git "):]
        generated.append(("%s, %s between git and the verb" % (command, where), spelled))
        for shape, form in wrappings():
            if shape == "bare":
                continue
            generated.append(
                ("%s, %s between git and the verb, %s" % (command, where, shape),
                 form % spelled))

    for command in VERBS:
        spelled = brace_expanded(command)
        if not spelled:
            continue
        generated.append(("%s, argument list brace-expanded" % command, spelled))
        for shape, form in wrappings():
            if shape == "bare":
                continue
            generated.append(("%s, argument list brace-expanded, %s" % (command, shape),
                              form % spelled))

    # A quoted argument on an otherwise bare invocation. The quoted text
    # is what the guard must not read as permission, and the invocation
    # around it is what the guard must still refuse.
    for command, (where, form) in itertools.product(VERBS, quoted_arguments(linked)):
        generated.append(("%s, %s" % (command, where), form % command))

    for command in VERBS:
        for shape, spelled in glued_to_each_flag(command):
            generated.append(("%s, %s" % (command, shape), spelled))
            for wrapper, form in wrappings():
                if wrapper == "bare":
                    continue
                generated.append(("%s, %s, %s" % (command, shape, wrapper),
                                  form % spelled))

    # Two invocations in one string, where one of them qualifies and the
    # other does not. A `-C` vouches for the invocation it is written on
    # and for no other, and this crossing is what says so. It is here
    # because arming the guard put it here: deleting `&` from the
    # boundary set is a fail-open that the generator above could not
    # produce a string for, since every string above carries one git.
    separators = [("&&", " && "), ("semicolon", " ; "), ("newline", "\n"),
                  ("pipe", " | "), ("or", " || "), ("background", " & ")]
    for command, (joined, separator) in itertools.product(VERBS, separators):
        good = qualified(command, linked)
        generated.append(("%s, qualifying then bare across %s" % (command, joined),
                          good + separator + command))
        generated.append(("%s, bare then qualifying across %s" % (command, joined),
                          command + separator + good))

    # The permission half, generated the same way. These must stay allowed:
    # a guard that pays for its coverage by refusing correct work is a
    # guard the operator turns off.
    for command, (shape, form) in itertools.product(VERBS, wrappings()):
        generated.append(("%s, -C a worktree, %s" % (command, shape),
                          form % qualified(command, linked)))

    # A worktree whose path contains a space can only be written quoted,
    # so these say that blanking quoted spans for detection has not cost
    # the guard the ability to read a path. A guard that refuses these is
    # a guard the operator turns off, and the fix for the quoted-argument
    # hole is precisely the fix that could break them.
    for command, (shape, form) in itertools.product(VERBS, wrappings()):
        generated.append(("%s, -C a worktree whose path has a space, %s" % (command, shape),
                          form % qualified(command, spaced)))

    # A qualifying invocation that also carries a quoted argument talking
    # about git. The quoted text grants nothing, and it must take nothing
    # away either.
    for command in VERBS:
        good = qualified(command, linked)
        generated.append(("%s, -C a worktree plus a quoted mention of the idiom" % command,
                          '%s "see git -C %s log first"' % (good, linked)))

    for command, (joined, separator) in itertools.product(VERBS, separators):
        good = qualified(command, linked)
        generated.append(("%s, both invocations qualifying across %s" % (command, joined),
                          good + separator + good))

    # Two invocations sharing one span, which the crossing above cannot
    # produce because it always puts a separator between them.
    generated.extend(two_in_one_span(linked, stub))

    # The strings this branch means to allow and the trunk refuses.
    generated.extend(relaxations(checkout, verbnamed))

    return generated


# ---------------------------------------------------------------------------
# Running the two halves.
# ---------------------------------------------------------------------------

def drain(directory):
    """The argument vectors the stub recorded, one file each, then emptied."""
    invocations = []
    for name in sorted(os.listdir(directory)):
        path = os.path.join(directory, name)
        with open(path, "r", encoding="utf-8", errors="replace") as handle:
            invocations.append(handle.read().splitlines())
        os.remove(path)
    return invocations


def run_in_shell(bash, command, sandbox, directory, environment):
    """Run one string and return the argument vectors git received.

    One shell per string rather than a batch of them. A generated string
    is allowed to be a syntax error, and a syntax error aborts the rest
    of a script it shares, so batching would silently stop observing
    after the first stray double semicolon the generator produced.

    A backgrounded invocation can outlive the shell that started it, so
    the recording directory is drained after a short settling pause
    rather than the instant the shell returns.
    """
    drain(directory)
    try:
        subprocess.run([bash, "--noprofile", "--norc", "-c", command],
                       cwd=sandbox, env=environment, capture_output=True,
                       text=True, timeout=30)
    except subprocess.TimeoutExpired:
        return None
    time.sleep(SETTLE)
    return drain(directory)


class Shells:
    """A pool of shells, one recording directory per worker thread.

    A shell costs about a quarter of a second to start on Windows and the
    generator produces thousands of strings, so the runs go in parallel.
    Each thread needs its own recording directory, because the directory
    is how the stub reports and two threads sharing one would report each
    other's invocations.
    """

    def __init__(self, bash, sandbox, root, environment, workers):
        self.bash = bash
        self.sandbox = sandbox
        self.root = root
        self.environment = environment
        self.workers = workers
        self.lock = threading.Lock()
        self.records = {}

    def record_for_this_thread(self):
        name = threading.current_thread().name
        with self.lock:
            if name not in self.records:
                directory = os.path.join(self.root, "record-%d" % len(self.records))
                os.makedirs(directory, exist_ok=True)
                self.records[name] = directory
            return self.records[name]

    def run(self, command):
        directory = self.record_for_this_thread()
        environment = dict(self.environment)
        environment["GIT_STUB_DIR"] = directory
        return run_in_shell(self.bash, command, self.sandbox, directory, environment)


def main():
    bash = shutil.which("bash")
    if not bash:
        print("no bash on PATH: this harness needs a real shell to be the oracle")
        return 1

    root = tempfile.mkdtemp(prefix="guard-against-a-real-shell-")
    failures = []
    regressions = []
    shapes = {}
    over_refusals = 0
    relaxed = 0
    total = 0
    reached = 0
    trunk = None
    try:
        checkout, linked, spaced, verbnamed = build_repo(root)

        stubdir = os.path.join(root, "stub")
        os.makedirs(stubdir)
        stub = os.path.join(stubdir, "git")
        with open(stub, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(STUB)
        os.chmod(stub, 0o755)

        sandbox = os.path.join(root, "sandbox")
        os.makedirs(sandbox)

        # A second copy of the stub in the shells' working directory, so
        # that `./git` is a real command rather than a string the shell
        # cannot run. `git.exe` needs no copy: on this platform the
        # extension names the same file.
        relative_stub = os.path.join(sandbox, "git")
        shutil.copyfile(stub, relative_stub)
        os.chmod(relative_stub, 0o755)
        record = os.path.join(root, "record-probe")
        os.makedirs(record)

        environment = dict(os.environ)
        environment["GIT_STUB_DIR"] = record
        environment["PATH"] = stubdir + os.pathsep + environment.get("PATH", "")

        # The stub has to be the git the shell finds, or every comparison
        # below is a comparison with nothing.
        probe = run_in_shell(bash, "git rev-parse --is-inside-work-tree",
                             sandbox, record, environment)
        if not probe:
            print("the recording stub is not first on PATH: nothing would be observed")
            return 1

        guard = load_guard()

        # The deployed guard, asked from the sandbox the shells run in.
        # That directory is no worktree, which is the situation the guard
        # exists for: a command whose working directory nobody can vouch
        # for.
        trunk_path = fetch_trunk_guard(root)
        if trunk_path:
            trunk = Trunk(load_module(trunk_path, "trunk_deny_destructive_git"), sandbox)

        kinds = {}
        kinds_lock = threading.Lock()

        def kind_of(directory):
            with kinds_lock:
                if directory in kinds:
                    return kinds[directory]
            answer = guard.worktree_kind(directory)
            with kinds_lock:
                kinds[directory] = answer
            return answer

        shells = Shells(bash, sandbox, root, environment, WORKERS)

        def examine(entry):
            name, command = entry
            allowed = guard.decide(command) is None
            trunk_refused = bool(trunk and allowed and trunk.refuses(command))
            invocations = shells.run(command)
            if invocations is None:
                return name, command, allowed, None, trunk_refused
            offences = []
            for argv in invocations:
                directory, subcommand, arguments = read_argv(argv)
                label = dangerous(subcommand, arguments)
                if not label:
                    continue
                if directory and os.path.isabs(directory) and kind_of(directory) == "linked":
                    continue
                offences.append((label, argv))
            return name, command, allowed, offences, trunk_refused

        with concurrent.futures.ThreadPoolExecutor(max_workers=WORKERS) as pool:
            for name, command, allowed, offences, trunk_refused in pool.map(
                    examine, strings(checkout, linked, spaced, verbnamed, stub)):
                total += 1
                if declares_relaxation(name):
                    # Held to both halves of what it claims. A string the
                    # trunk no longer refuses has stopped being a
                    # relaxation, and a string this branch refuses is the
                    # fix gone missing.
                    if not allowed:
                        failures.append((name, command,
                                         "declared as a relaxation and refused by this branch",
                                         None))
                    elif trunk is not None and not trunk_refused:
                        failures.append((name, command,
                                         "declared as a relaxation and allowed by the deployed "
                                         "guard too, so it relaxes nothing", None))
                    else:
                        relaxed += 1
                elif trunk_refused:
                    regressions.append((name, command))
                if offences is None:
                    failures.append((name, command, "the shell did not finish", None))
                    continue
                if offences:
                    reached += 1
                if offences and allowed:
                    failures.append((name, command, offences[0][0], offences[0][1]))
                elif not offences and not allowed:
                    over_refusals += 1
                    shape = name.split(", ", 1)[1] if ", " in name else name
                    shapes[shape] = shapes.get(shape, 0) + 1
    finally:
        shutil.rmtree(root, ignore_errors=True)

    print("generated %d strings" % total)
    print("%d of them reached the stub with a deny-set verb and no qualifying -C"
          % reached)
    print("%d were refused although the shell ran nothing in the deny set "
          "(over-refusal, not a failure)" % over_refusals)
    for shape, count in sorted(shapes.items(), key=lambda pair: -pair[1])[:12]:
        print("    %4d  %s" % (count, shape))
    print("%d of them are declared relaxations, allowed here and refused by the "
          "deployed guard" % relaxed)
    if trunk is None:
        print("the deployed guard at %s could not be read, so no string was "
              "compared against it" % TRUNK_REF)
    else:
        print("every string was also put to the deployed guard at %s (%d of them "
              "it could not read)" % (TRUNK_REF, trunk.errors))
    print()

    for name, command in regressions:
        print("REGRESSION %s" % name)
        print("     command: %r" % command)
        print("     the deployed guard refuses this and this branch allows it")
    if regressions:
        print()

    if failures:
        for name, command, label, argv in failures:
            print("FAIL %s" % name)
            print("     command: %r" % command)
            print("     git received: %s (%r)" % (label, argv))
        print()

    if failures or regressions:
        print("%d fail-open(s) and %d regression(s) against %s, out of %d "
              "generated strings" % (len(failures), len(regressions), TRUNK_REF, total))
        return 1

    if trunk is None:
        print("no fail-open in %d generated strings, but the comparison against "
              "%s did not run" % (total, TRUNK_REF))
        return 1

    if trunk.errors:
        print("the deployed guard could not read %d of the %d strings, so those "
              "were compared against nothing" % (trunk.errors, total))
        return 1

    print("no fail-open and no regression against %s in %d generated strings"
          % (TRUNK_REF, total))
    return 0


if __name__ == "__main__":
    sys.exit(main())
