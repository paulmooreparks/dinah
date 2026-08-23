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
the command actually did. That was improvised by hand and thrown away
three times. This file is that oracle, kept.

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

The oracle that reads the recorded arguments is deliberately not the
guard's own code. It walks an argument vector, which is a list the shell
already split, so it needs no shell grammar and shares no reasoning with
the thing it is checking. A checker built out of the checked would agree
with it about everything, including its mistakes.

SAFETY. Every generated string is git and nothing else, wrapped in
punctuation. `git` is the stub, so no repository is touched, and the
shell runs with its working directory in a temporary sandbox. The one
real repository the run builds is its own, under a temporary directory,
and it exists only so that a `-C` can name a genuine linked worktree.
"""

import concurrent.futures
import itertools
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

# How many shells run at once. The work is a process start rather than
# arithmetic, so more workers than cores still pays.
WORKERS = 12

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


def load_guard():
    """The guard module itself, imported rather than copied.

    The harness reads the real `decide`, so a change to the guard's early
    bail or to its patterns is a change to what this harness tests.
    """
    spec = importlib.util.spec_from_file_location("deny_destructive_git", HOOK)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def git(*args, cwd=None):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True)


def build_repo(root):
    """A main checkout with one commit and one linked worktree beside it."""
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
    return main, linked


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
    """Punctuation written between `git` and its subcommand.

    This is the position that leaked the fourth cycle: a lexer split `>`
    out as a word of its own and the reader underneath called that `>`
    the subcommand, cleared the command, and let `git >/dev/null reset
    --hard` run. Every one of these is a real invocation of the verb that
    follows it, so a guard reading any of them as anything else is wrong.
    """
    return [
        ("stdout redirection", ">/dev/null "),
        ("stderr redirection", "2>/dev/null "),
        ("stderr onto stdout", "2>&1 "),
        ("stdin redirection", "</dev/null "),
        ("append redirection", ">>/dev/null "),
        ("descriptor duplication", "3>&1 "),
        ("line continuation", "\\\n  "),
        ("two continuations", "\\\n \\\n "),
        ("redirection then continuation", ">/dev/null \\\n "),
        ("global option", "--no-pager "),
        ("global option then redirection", "--no-pager >/dev/null "),
        ("configuration override", "-c core.pager=cat "),
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


def strings(linked):
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
        spelled = "git " + insert + command[len("git "):]
        generated.append(("%s, %s between git and the verb" % (command, where), spelled))
        for shape, form in wrappings():
            if shape == "bare":
                continue
            generated.append(
                ("%s, %s between git and the verb, %s" % (command, where, shape),
                 form % spelled))

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

    for command, (joined, separator) in itertools.product(VERBS, separators):
        good = qualified(command, linked)
        generated.append(("%s, both invocations qualifying across %s" % (command, joined),
                          good + separator + good))

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
    shapes = {}
    over_refusals = 0
    total = 0
    reached = 0
    try:
        checkout, linked = build_repo(root)

        stubdir = os.path.join(root, "stub")
        os.makedirs(stubdir)
        stub = os.path.join(stubdir, "git")
        with open(stub, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(STUB)
        os.chmod(stub, 0o755)

        sandbox = os.path.join(root, "sandbox")
        os.makedirs(sandbox)
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
            invocations = shells.run(command)
            if invocations is None:
                return name, command, allowed, None
            offences = []
            for argv in invocations:
                directory, subcommand, arguments = read_argv(argv)
                label = dangerous(subcommand, arguments)
                if not label:
                    continue
                if directory and os.path.isabs(directory) and kind_of(directory) == "linked":
                    continue
                offences.append((label, argv))
            return name, command, allowed, offences

        with concurrent.futures.ThreadPoolExecutor(max_workers=WORKERS) as pool:
            for name, command, allowed, offences in pool.map(examine, strings(linked)):
                total += 1
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
    print()

    if failures:
        for name, command, label, argv in failures:
            print("FAIL %s" % name)
            print("     command: %r" % command)
            print("     git received: %s (%r)" % (label, argv))
        print()
        print("%d fail-open(s) out of %d generated strings" % (len(failures), total))
        return 1

    print("no fail-open in %d generated strings" % total)
    return 0


if __name__ == "__main__":
    sys.exit(main())
