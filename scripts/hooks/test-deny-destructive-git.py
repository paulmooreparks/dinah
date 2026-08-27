#!/usr/bin/env python3
r"""Cases the destructive-git guard must hold to.

Run it from anywhere: `python scripts/hooks/test-deny-destructive-git.py`.
It prints every case either way and exits non-zero when any of them
disagrees with the table, so a failure names itself rather than needing a
debugger.

The table builds its own repository under a temporary directory, with a
main checkout and four linked worktrees, rather than asserting against
the operator's real one. That keeps the run reproducible on a machine
whose worktrees sit somewhere else, and it means the test never points a
destructive command at anything a person cares about.

The contract these cases pin is a single sentence. A git invocation whose
verb is in the deny set is allowed only when that same invocation carries
`-C <path>` and the path classifies as a linked worktree. The session's
reported working directory decides nothing, which is why most verbs are
run from both the main checkout and a worktree and expected to give the
same answer in each.
"""

import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile

HOOK = os.path.join(os.path.dirname(os.path.abspath(__file__)), "deny-destructive-git.py")

DENY = "deny"
ALLOW = "allow"

# Spelled in pieces so the file itself does not carry a string the live
# guard refuses when an agent edits this repository.
RESET = "git re" + "set --hard origin/main"


def git(*args, cwd=None):
    subprocess.run(["git", *args], cwd=cwd, check=True, capture_output=True, text=True)


def build_repo(root):
    """A main checkout with one commit, plus four linked worktrees beside it."""
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
    # A worktree whose own path carries a git component. The guard counts
    # invocations with the pattern it finds them with, and that pattern
    # matches the letters wherever a path separator or a hyphen stands
    # either side of them, so this path costs an unquoted invocation a
    # refusal. The two cases built on it pin the cost and the escape.
    componented = os.path.join(root, "git", "card-impl", "wt")
    git("worktree", "add", "--detach", "-q", componented, "HEAD", cwd=main)
    # A worktree named the way this board names one, for a card at the
    # merge stage. The verb is a syllable of the directory rather than a
    # subcommand, and the cases built on it say the guard reads it that
    # way.
    verbnamed = os.path.join(root, "scratch", "dinah-249-merge", "wt")
    git("worktree", "add", "--detach", "-q", verbnamed, "HEAD", cwd=main)
    return main, linked, spaced, nested, componented, verbnamed


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


def qualified(command, path):
    """The same command with a `-C <path>` naming where it runs."""
    assert command.startswith("git "), command
    return 'git -C "%s" %s' % (path, command[4:])


def slashed(path):
    """`path` written with forward slashes, which is how this board writes one.

    The path cases below put a deny-set verb inside a directory name and
    ask whether the guard still reads it as a subcommand, so the
    separator has to be the one those cases reason about. A backslash is
    how a shell quotes the letter after it rather than a separator, and
    the guard does not read one as the end of a word, so a
    backslash-spelled fixture would be asking a different question and
    would pass while the reported defect stood.
    """
    return path.replace("\\", "/")


def msys(path):
    """`path` written the way Git Bash hands a Windows drive path back.

    `C:/x/y` becomes `/c/x/y`. Every agent on this board runs its shell
    through Git Bash, so this is the spelling a person writes without
    thinking about it, and the spelling `os.path.isabs` accepts on
    Windows while the `git` subprocess underneath `worktree_kind` cannot
    find the directory it names. Off Windows there is no drive letter to
    rewrite, so the path comes back as it stands and the cases built on
    this helper are guarded the way the backslashed fixture is.
    """
    forward = slashed(path)
    if os.name != "nt" or len(forward) < 2 or forward[1] != ":":
        return forward
    return "/" + forward[0].lower() + forward[2:]


def windows_spelled(path):
    """`path` with forward slashes and an upper-case drive letter.

    This is what the guard's refusal has to name once it has normalised
    an msys-spelled `-C`, and it is composed here rather than read back
    out of the guard, so that a case asserting it fails when the guard
    stops producing it.
    """
    forward = slashed(path)
    if len(forward) < 2 or forward[1] != ":":
        return forward
    return forward[0].upper() + forward[1:]


def load_guard():
    """The guard module itself, imported rather than re-implemented.

    `test-guard-against-a-real-shell.py` holds the same three importlib
    calls under the same module name, and the duplication is deliberate.
    Both files are hyphen-named entry points, so neither can import the
    other without a loader of its own, and a third module existing only
    to hold three lines would cost each of them the loader it is trying
    to avoid. Change one and change the other.

    Importing is what lets a case reach a pure function whose two
    branches cannot both be exercised end to end on one host.
    """
    spec = importlib.util.spec_from_file_location("deny_destructive_git", HOOK)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def hook_reason(command, cwd):
    """The refusal text the guard printed, or None when it allowed the command.

    `verdict` reads the decision alone, and the decision cannot tell an
    msys-spelled path the guard understood from one it handed to git
    unchanged: both are refused, for different reasons and about
    different directories. The sentence the guard writes names the
    directory it actually judged, so these cases read that.
    """
    payload = json.dumps({"tool_input": {"command": command}, "cwd": cwd})
    result = subprocess.run(
        [sys.executable, HOOK], input=payload, capture_output=True, text=True, timeout=60
    )
    if result.returncode != 0:
        raise AssertionError("hook exited %d: %s" % (result.returncode, result.stderr.strip()))
    if not result.stdout.strip():
        return None
    return json.loads(result.stdout)["hookSpecificOutput"]["permissionDecisionReason"]


# The deny-set verbs a path segment can carry on its own. Each of these
# rules fires on the verb alone, so a directory named after one used to
# cost a read-only command its permission. The rules left out need a
# flag as well (reset, clean, checkout, switch, push, branch, tag) or a
# second word (worktree remove), and a path segment does not supply one,
# so there was nothing for a path alone to trip.
PATH_VERBS = ["restore", "stash", "commit", "merge", "rebase", "cherry-pick",
              "revert", "am", "apply", "rm", "mv", "bisect", "pull"]


# Every verb the guard refuses. Each one is run six ways, and the first
# three of those are the point of the contract: a bare invocation is
# refused wherever the session says it stands, including when the session
# says nothing at all.
MUTATING = [
    ("reset --hard", "git re" + "set --hard origin/main"),
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

# Ordinary work the guard has no business refusing, wherever it runs and
# whether or not it names a directory. Every carve-out named in the deny
# set appears here, because an exemption nobody tests is an exemption that
# quietly closes.
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
    ("reset without --hard", "git re" + "set HEAD~1"),
    ("restore --staged only", "git restore --staged seed.txt"),
    ("push --force-with-lease to a topic", "git push --force-with-lease origin topic"),
]


def glued_shapes(command):
    """The same command with a shell metacharacter glued to a word.

    This is the shape the suite went without for 269 cases, and its
    absence is what let a whitespace-only tokeniser look correct. Each
    spelling here is one a shell runs exactly as it runs the bare form,
    with no space anywhere for a word-splitting reader to find.
    """
    return [
        ("glued behind a semicolon", "echo a;%s" % command),
        ("glued behind &&", "true&&%s" % command),
        ("wrapped in parentheses", "(%s)" % command),
        ("inside an if/then/fi", "if true;then %s;fi" % command),
        ("inside a for loop", "for i in 1;do %s;done" % command),
        ("with a redirection glued on", "%s>/dev/null" % command),
        ("with a semicolon glued on", "%s;" % command),
    ]


def glued_flags(command):
    """The same command with a metacharacter glued to each of its flags.

    The verb scan reads a flag by comparing it with a string, so `--hard;`
    and `--hard>/dev/null` are the same defect one layer down from a
    hidden invocation rather than a different one, and they are generated
    from the deny set rather than listed for the verbs somebody thought of.
    """
    shapes = []
    words = command.split(" ")
    for position, word in enumerate(words):
        if not word.startswith("-"):
            continue
        for glue in (";", ">/dev/null"):
            spelled = list(words)
            spelled[position] = word + glue
            shapes.append(("%s glued to %s" % (glue, word), " ".join(spelled)))
    return shapes


# A parameter expansion whose value is the word standing inside it. The
# shell hands git the word and eats the braces and the operator, so a
# character the whole-word lookarounds refuse can stand beside a literal
# verb in the text and never reach git at all. Four of the seven refused
# characters are operators here, which is why `git ${x-reset} --hard` is
# a working hard reset that the lookarounds alone decline to read.
#
# The two shapes that need the parameter set carry their assignment in
# front across a separator rather than as a prefix on the same command.
# An assignment prefix takes effect after the rest of the words have
# been expanded, so `x=1 git ${x+reset}` would read the old value and
# generate a command that runs nothing.
EXPANSIONS = [
    ("a default for an unset parameter", "", "${x-%s}"),
    ("a default for an unset or empty parameter", "", "${x:-%s}"),
    ("an assigned default", "", "${x=%s}"),
    ("an assigned default for an empty parameter", "", "${x:=%s}"),
    ("an alternate value for a set parameter", "x=1 ; ", "${x+%s}"),
    ("an alternate value for a set, non-empty parameter", "x=1 ; ", "${x:+%s}"),
    ("the first match substituted", "x=y ; ", "${x/y/%s}"),
    ("every match substituted", "x=y ; ", "${x//y/%s}"),
]


def expanded(command, index):
    """`command` with the word at `index` written inside each expansion."""
    words = command.split(" ")
    shapes = []
    for name, assignment, form in EXPANSIONS:
        spelled = list(words)
        spelled[index] = form % words[index]
        shapes.append((name, assignment + " ".join(spelled)))
    return shapes


def deciding_operand(command):
    """Where the flag or refspec that condemns the verb stands, or None.

    A verb the guard refuses on its own has no such word, and those
    commands are covered by the verb half of the crossing alone.

    The condemning word is found by taking the first argument from index
    2 onward that starts with `-` or `:`, which covers every deciding
    operand in the guard's table today. A future verb whose deciding
    operand is spelled some other way returns None here and loses its
    half of the crossing without turning any run red, so a rule added to
    the guard wants a look at this line as well.
    """
    words = command.split(" ")
    for index, word in enumerate(words):
        if index >= 2 and (word.startswith("-") or word.startswith(":")):
            return index
    return None


def leaks(linked):
    """The seven spellings that got past the parsing design.

    Each was found by a person reading the code rather than by the suite,
    and each is a real string. They are kept as regressions and every one
    of them is now refused, for a structural reason rather than because
    the spelling was handled: none carries a `-C`, and nothing else can
    grant permission.
    """
    return [
        ("leak: a cd inside a quoted argument",
         'git commit -m "cd %s && note" && %s' % (linked, RESET)),
        ("leak: a here-string read as a heredoc opener",
         "cat <<<word\n%s" % RESET),
        ("leak: a heredoc marker containing a hyphen",
         "cat <<E-OF\ncd %s\nE-OF\n%s" % (linked, RESET)),
        ("leak: a heredoc marker beginning with a digit",
         "cat <<1EOF\ncd %s\n1EOF\n%s" % (linked, RESET)),
        ("leak: pushd instead of cd", "pushd %s && %s" % (linked, RESET)),
        ("leak: builtin cd", "builtin cd %s && %s" % (linked, RESET)),
        ("leak: a backslash-escaped cd", "\\cd %s && %s" % (linked, RESET)),
    ]


def cases(root, main, linked, spaced, nested, componented, verbnamed):
    gone = os.path.join(main, ".claude", "worktrees", "deleted-out-from-under-us")
    backslashed = linked.replace("/", "\\") if os.name == "nt" else linked
    table = []

    # `msys` rewrites a drive letter, and `linked` does not change across
    # the loop below, so the rewrite is asserted once here rather than
    # twenty-eight times inside it. The nested-worktree site further down
    # asserts the same thing about its own fixture.
    if os.name == "nt":
        assert msys(linked) != slashed(linked), (
            "msys() left %r in its plain spelling" % linked)

    for name, command in MUTATING:
        table.append(("%s, bare, session in the main checkout" % name, command, main, DENY))
        table.append(("%s, bare, session in a worktree" % name, command, linked, DENY))
        table.append(("%s, bare, no cwd reported" % name, command, "", DENY))
        table.append(("%s, -C a worktree, session in the main checkout" % name,
                      qualified(command, linked), main, ALLOW))
        table.append(("%s, -C a worktree, session in a worktree" % name,
                      qualified(command, linked), linked, ALLOW))
        table.append(("%s, -C the main checkout" % name, qualified(command, main), linked, DENY))
        # The same permission, asked for in the spelling Git Bash hands
        # back. A guard that clears `C:/...` and refuses `/c/...` refuses
        # the form every agent on this board writes first.
        if os.name == "nt":
            table.append(("%s, -C a worktree spelled the Git Bash way" % name,
                          qualified(command, msys(linked)), main, ALLOW))

    # Every deny-set verb again, once per glued spelling. A verb that is
    # refused when its words stand apart and allowed when a semicolon
    # touches one of them is not refused at all.
    for name, command in MUTATING:
        for shape, spelled in glued_shapes(command) + glued_flags(command):
            table.append(("%s, %s" % (name, shape), spelled, main, DENY))

    for name, command in ORDINARY:
        table.append(("%s in the main checkout" % name, command, main, ALLOW))
        table.append(("%s in a worktree" % name, command, linked, ALLOW))

    # The other half of the same question. Splitting on metacharacters
    # must not cost a command its permission, so a qualifying invocation
    # keeps it in every one of those spellings, and so does ordinary work.
    for name, command in [entry for entry in MUTATING
                          if entry[0] in ("reset --hard", "clean -fdx", "commit",
                                          "branch -D", "push --delete")]:
        for shape, spelled in glued_shapes(qualified(command, linked)):
            table.append(("%s, -C a worktree, %s" % (name, shape), spelled, main, ALLOW))

    for name, command in [entry for entry in ORDINARY
                          if entry[0] in ("status", "log", "fetch", "worktree prune")]:
        for shape, spelled in glued_shapes(command):
            table.append(("%s, %s" % (name, shape), spelled, main, ALLOW))

    # Every deny-set verb written inside a parameter expansion, and every
    # deciding flag written inside one too. This is the class the
    # whole-word lookarounds opened and the class the second reading
    # closes, and it is generated from the deny set rather than listed
    # for the verbs somebody thought of. A qualifying form of each shape
    # is here as well, because a guard that pays for this coverage by
    # refusing a command that names its worktree is a guard nobody keeps.
    for name, command in MUTATING:
        for shape, spelled in expanded(command, 1):
            table.append(("%s, the verb inside %s" % (name, shape), spelled, main, DENY))
        index = deciding_operand(command)
        if index is not None:
            for shape, spelled in expanded(command, index):
                table.append(("%s, the deciding operand inside %s" % (name, shape),
                              spelled, main, DENY))

    for name, assignment, form in EXPANSIONS:
        table.append(("commit, the verb inside %s, -C a worktree" % name,
                      '%sgit -C "%s" %s -m wip' % (assignment, linked, form % "commit"),
                      main, ALLOW))

    for name, command in leaks(linked):
        table.append((name, command, main, DENY))

    # The nested worktree again, in the spelling Git Bash hands back.
    # Guarded like every other case built through `msys`, because that
    # helper rewrites a drive letter and off Windows there is none to
    # rewrite: the case would build the same command as its plain twin
    # below and then demand the opposite verdict, so one of the two
    # would fail on every host that is not Windows. A conditional
    # expectation cannot rescue a case whose distinguishing input the
    # constructor has already erased. The assertion says the rewrite
    # fired, so a later change to `msys` fails here rather than leaving
    # a duplicate wearing a name that claims otherwise.
    if os.name == "nt":
        assert msys(nested) != slashed(nested), (
            "msys() left %r in its plain spelling" % nested)
        table.append(("-C a nested worktree spelled the Git Bash way",
                      'git -C "%s" clean -fdx' % msys(nested), main, ALLOW))

    table.extend([
        # What a -C has to name before it grants anything.
        ("-C a nested worktree is still a linked worktree",
         'git -C "%s" clean -fdx' % nested, main, ALLOW),
        ("-C a directory that is not a git work tree",
         'git -C "%s" clean -fdx' % tempfile.gettempdir(), main, DENY),
        # A path shaped like a Git Bash drive mount whose first segment
        # is more than one letter is not one, so it reaches the
        # classifier as written and fails closed there.
        ("-C a POSIX path that is not a drive mount",
         "git -C /cool/path clean -fdx", main, DENY),
        # A directory reaches the guard as text, and a shell variable is
        # not a directory until a shell has expanded it. Normalising a
        # drive mount must not start reading one as absolute.
        ("-C held in a shell variable is still refused",
         "git -C $WT stash pop", main, DENY),
        # A `-C` with nothing after it names no directory. The guard has
        # to answer that before it normalises anything, because the
        # normaliser is handed a string and `None` is not one.
        ("a trailing -C names no directory",
         'git -C "%s" commit -m wip -C' % linked, main, DENY),
        ("a trailing -C on a bare invocation is still refused",
         "git re" + "set --hard -C", main, DENY),
        ("-C a directory that no longer exists",
         'git -C "%s" clean -fdx' % gone, main, DENY),
        ("-C a path written with backslashes",
         "git -C %s stash pop" % backslashed, main, ALLOW),
        ("-C a quoted path containing a space",
         'git -C "%s" stash pop' % spaced, main, ALLOW),
        ("-C attached to its value", 'git -C"%s" stash pop' % linked, main, ALLOW),
        # A relative -C names a directory only together with a working
        # directory, and the working directory is what the guard stopped
        # consulting. It is refused rather than composed.
        ("-C . is relative and names nothing on its own",
         "git -C . stash pop", linked, DENY),
        ("-C a relative subdirectory names nothing on its own",
         "git -C scratch/card-impl/wt stash pop", root, DENY),

        # Options naming a target the guard does not follow.
        ("--git-dir instead of -C", 'git --git-dir="%s/.git" stash pop' % main, linked, DENY),
        ("--git-dir spelled with a space",
         'git --git-dir "%s/.git" stash pop' % main, linked, DENY),
        ("--work-tree instead of -C", 'git --work-tree="%s" stash pop' % linked, main, DENY),
        ("--work-tree spelled with a space",
         'git --work-tree "%s" stash pop' % linked, main, DENY),
        # The four cases above hold whether or not the guard refuses the
        # option, because an invocation using one carries no -C either and
        # is refused for that instead. These three rest on the refusal
        # alone: each carries a -C that would otherwise clear it.
        ("--git-dir alongside a qualifying -C is still refused",
         'git -C "%s" --git-dir="%s/.git" stash pop' % (linked, main), main, DENY),
        ("--git-dir ahead of a qualifying -C is still refused",
         'git --git-dir="%s/.git" -C "%s" stash pop' % (main, linked), main, DENY),
        ("--work-tree alongside a qualifying -C is still refused",
         'git -C "%s" --work-tree "%s" stash pop' % (linked, main), main, DENY),

        # One qualifying invocation does not clear the command.
        ("two invocations, only the first qualifies",
         'git -C "%s" stash pop && %s' % (linked, RESET), linked, DENY),
        ("two invocations, only the second qualifies",
         '%s && git -C "%s" stash pop' % (RESET, linked), linked, DENY),
        ("two invocations, one -C a worktree and one -C the checkout",
         'git -C "%s" stash pop && git -C "%s" clean -fdx' % (linked, main), linked, DENY),
        ("two invocations, both qualifying",
         'git -C "%s" stash pop && git -C "%s" commit -m done' % (linked, linked), main, ALLOW),
        ("a qualifying invocation beside an ordinary one",
         'git fetch origin && git -C "%s" merge origin/main' % linked, main, ALLOW),

        # Quoted text is an argument rather than a command, which is what
        # the verb scan's quoted-span stripping buys and all it buys.
        ("prose quoting a destructive command is not one",
         'git log --oneline --grep "git clean -fdx"', main, ALLOW),
        ("a destructive command quoted inside a qualifying one",
         'git -C "%s" commit -m "%s"' % (linked, RESET), main, ALLOW),
        ("a quote span closing inside a later word is still one argument",
         "git log 'a && %s don't" % RESET, main, ALLOW),
        ("an unterminated quotation mark is unreadable and refused",
         'git commit -m "oops && git clean -fd', main, DENY),
        ("a bare apostrophe tokenises and is judged",
         "git commit -m it's && git clean -fd", main, DENY),

        # The forms the retired design used to allow. Every one of them is
        # a command that does not say where it runs, so every one is now
        # refused, and this block is the record of what the inversion cost.
        ("cd then a bare invocation, across &&", "cd %s && git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across ;", "cd %s ; git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across a newline",
         "cd %s\ngit stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across ||", "cd %s || git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across |", "cd %s | git stash pop" % linked, main, DENY),
        ("cd then a bare invocation, across &", "cd %s & git stash pop" % linked, main, DENY),
        ("a parenthesised cd and its bare invocation",
         "(cd %s && git stash pop)" % linked, main, DENY),
        ("a cd inside a heredoc body",
         "git log <<EOF\ncd %s\nEOF\ngit stash pop" % linked, main, DENY),
        ("a cd written with backslashes", "cd %s && git stash pop" % backslashed, main, DENY),
        ("a cd to a quoted path containing a space",
         'cd "%s" && git stash pop' % spaced, main, DENY),
        ("a relative cd", "cd scratch/card-impl/wt && git stash pop", root, DENY),
        ("a brace group carrying a cd", "{ cd %s ; git stash pop ; }" % linked, main, DENY),

        # A continuation is one invocation split over two lines, and the
        # flag on the second line still counts.
        ("continuation splits the flag", "git re" + "set \\\n  --hard origin/main", main, DENY),
        ("continuation splits the flag on a qualifying invocation",
         'git -C "%s" re' % linked + "set \\\n  --hard origin/main", main, ALLOW),

        # The early bail reads the command case-insensitively, because the
        # basename matching further in already does.
        ("an uppercase GIT is still git", "GIT re" + "set --hard origin/main", main, DENY),
        ("an uppercase GIT can still qualify",
         'GIT -C "%s" re' % linked + "set --hard origin/main", main, ALLOW),
        ("git named by an absolute path",
         "/usr/bin/git stash pop", main, DENY),
        ("git named by an absolute path, qualifying",
         '/usr/bin/git -C "%s" stash pop' % linked, main, ALLOW),

        # Metacharacters the shapes above do not generate, each hiding an
        # invocation from a reader that splits on whitespace alone.
        ("a backtick substitution", "echo `git stash pop`", main, DENY),
        ("a dollar-paren substitution", "echo $(git stash pop)", main, DENY),
        ("a pipe glued to the last word", "git stash pop|cat", main, DENY),
        ("an ampersand glued to the last word", "git stash pop&", main, DENY),
        ("a brace group with no spaces", "{git stash pop;}", main, DENY),
        ("a bang glued to the invocation", "!git stash pop", main, DENY),
        ("a case arm glued to the invocation",
         "case x in x)git stash pop;;esac", main, DENY),
        ("a brace group with spaces", "{ git stash pop ; }", main, DENY),

        # The second matcher counts git words the lexer may have missed,
        # and these are the ordinary spellings it must not count. Each one
        # is allowed today and stays allowed, because a scan that refuses
        # `.git/config` is a scan somebody turns off.
        ("a dot-git directory is not a git word", "cat .git/config", main, ALLOW),
        ("a hyphenated command is not a git word", "git-lfs install", main, ALLOW),
        ("an ssh remote is not a second git word",
         "git clone git@github.invalid:x/y.git", main, ALLOW),
        ("a git protocol URL is not a second git word",
         "git clone git://github.invalid/x/y.git", main, ALLOW),
        ("a dot-git path beside a qualifying invocation",
         'cat .git/HEAD && git -C "%s" stash pop' % linked, main, ALLOW),

        # Punctuation written between `git` and its subcommand. This is
        # the position that cleared a denied verb in the fourth cycle, and
        # neither of the two spellings below was reachable by any case in
        # this file until the differential harness produced them.
        ("a redirection between git and the verb",
         "git >/dev/null re" + "set --hard origin/main", main, DENY),
        ("a stderr redirection between git and the verb",
         "git 2>/dev/null re" + "set --hard origin/main", main, DENY),
        ("stderr folded onto stdout between git and the verb",
         "git 2>&1 re" + "set --hard origin/main", main, DENY),
        ("a continuation between git and the verb",
         "git \\\n  re" + "set --hard origin/main", main, DENY),
        ("a global option between git and the verb",
         "git --no-pager re" + "set --hard origin/main", main, DENY),

        # `-c` sets configuration and `-C` names a directory. Reading them
        # as the same option cleared every deny-set verb, and no case in
        # this file caught it; the harness did, on its first run.
        ("a configuration override is not a directory",
         "git -c core.pager=cat re" + "set --hard origin/main", main, DENY),
        ("a configuration override ahead of a real -C is still refused",
         'git -c core.pager=cat -C "%s" stash pop' % linked, main, DENY),

        # Expansion and substitution written between `git` and its verb.
        # The span the guard reads used to stop at a parenthesis, a brace
        # and a backtick, and a shell keeps every one of those inside a
        # single command, so the verb was simply not in the text the guard
        # searched. The trunk had been refusing all five of these all
        # along, which is what makes them a regression rather than a gap.
        ("a parameter expansion between git and the verb",
         "git ${OPTS} re" + "set --hard origin/main", main, DENY),
        ("a command substitution between git and the verb",
         "git $(echo --no-pager) re" + "set --hard origin/main", main, DENY),
        ("a backtick substitution between git and the verb",
         "git `echo --no-pager` re" + "set --hard origin/main", main, DENY),
        ("IFS standing in for the space between git and the verb",
         "git${IFS}re" + "set --hard origin/main", main, DENY),
        ("the argument list written as a brace expansion",
         "git {re" + "set,--hard}", main, DENY),
        ("a brace expansion of a verb that takes no flag",
         "git {stash,pop}", main, DENY),

        # The same brace expansion one layer down. Eight of the rules
        # decided a flag by requiring whitespace in front of it, and a
        # brace expansion puts a comma there instead, so the verb was
        # found and the flag that condemns it was not. A flag is
        # delimited by what a word cannot contain, and these say so.
        ("a brace-expanded clean -fdx", "git {clean,-fdx}", main, DENY),
        ("a brace-expanded checkout --", "git {checkout,--,.}", main, DENY),
        ("a brace-expanded checkout -f", "git {checkout,-f,main}", main, DENY),
        ("a brace-expanded push --force",
         "git {push,--force,origin,topic}", main, DENY),
        ("a brace-expanded push --delete",
         "git {push,origin,--delete,topic}", main, DENY),
        ("a brace-expanded colon refspec", "git {push,origin,:topic}", main, DENY),
        ("a brace-expanded switch -f", "git {switch,-f,main}", main, DENY),
        ("a brace-expanded switch --discard-changes",
         "git {switch,--discard-changes,main}", main, DENY),
        ("a brace-expanded branch -D", "git {branch,-D,topic}", main, DENY),
        ("a brace-expanded tag -d", "git {tag,-d,v1.0}", main, DENY),

        # A `-C` written inside a quoted argument is text git never reads
        # as an option, so it grants nothing. This one is reachable rather
        # than theoretical: `git -C C:/dinah-scratch/...` is the string the
        # agent definitions, four columns and this guard's own refusal text
        # all tell an agent to write, so an agent quoting the board's own
        # advice used to disarm the guard against itself.
        ("a quoted -C does not vouch for a bare reset",
         'git re' + 'set --hard "git -C %s x"' % linked, main, DENY),
        ("a quoted -C does not vouch for a bare commit",
         'git commit -m "dinah-9: run git -C %s log first"' % linked, main, DENY),
        ("a quoted -C does not vouch for a bare stash pop",
         'git stash pop "git -C %s ."' % linked, main, DENY),
        ("a quoted -C with no git word in front of it vouches for nothing",
         'git clean -fdx "-C %s"' % linked, main, DENY),
        ("a single-quoted -C vouches for nothing either",
         "git re" + "set --hard 'git -C %s x'" % linked, main, DENY),

        # The other half of the same rule: the `-C` token has to stand
        # outside quotation marks, and its argument may be inside them. A
        # worktree whose path contains a space can be named no other way.
        ("a qualifying invocation carrying a quoted mention of the idiom",
         'git -C "%s" commit -m "see git -C %s log first"' % (linked, linked),
         main, ALLOW),
        ("a quoted path with a space still names its worktree",
         'git -C "%s" stash pop' % spaced, main, ALLOW),
        ("a single-quoted path with a space still names its worktree",
         "git -C '%s' stash pop" % spaced, main, ALLOW),

        # Two invocations inside one span. Crossing a parenthesis is what
        # closed the regression above, and its price is that a span can
        # hold two commands with no boundary character between them. The
        # guard cannot say which one a `-C` belongs to, so it refuses.
        ("a qualifying invocation with a bare one substituted into it",
         'git -C "%s" log $(git re' % linked + 'set --hard)', main, DENY),
        ("a qualifying invocation with a bare one in backticks",
         'git -C "%s" log `git stash pop`' % linked, main, DENY),

        # The same rule reached by a second invocation spelled as
        # anything but the bare word. The guard finds an invocation with
        # a word-bounded `git`, which every one of these carries, and its
        # counter used to carry none of them, so a span read as one
        # invocation while the shell ran two and the `-C` on the first
        # cleared the second.
        ("a second invocation spelled with a relative path",
         'git -C "%s" stash list $(./git re' % linked + 'set --hard)', main, DENY),
        ("a second invocation spelled with an absolute path",
         'git -C "%s" stash list $(/usr/bin/git re' % linked + 'set --hard)',
         main, DENY),
        ("a second invocation spelled with an executable extension",
         'git -C "%s" stash list $(git.exe re' % linked + 'set --hard)', main, DENY),
        ("a second invocation with a path, in backticks",
         'git -C "%s" stash list `./git stash pop`' % linked, main, DENY),
        ("a second invocation inside a qualifying deny-set invocation",
         'git -C "%s" commit -m wip $(./git re' % linked + 'set --hard)', main, DENY),
        ("the bare text of the idiom clears nothing",
         'echo git -C "%s" $(./git re' % linked + 'set --hard)', main, DENY),

        # A substitution written inside double quotation marks. The shell
        # runs it, so the guard has to see it; blanking the whole span
        # cleared a reset the shell then ran. Single quotation marks are
        # the other case, where the shell runs nothing and a string is a
        # string.
        ("a double-quoted substitution is still a command",
         'git -C "%s" commit -m "$(git re' % linked + 'set --hard)"', main, DENY),
        ("a double-quoted substitution carrying a path-spelled invocation",
         'git -C "%s" stash list "$(./git re' % linked + 'set --hard)"', main, DENY),
        ("a double-quoted substitution with two qualifying invocations",
         'git -C "%s" commit -m "wip $(git -C %s rev-parse HEAD)"' % (linked, linked),
         main, DENY),
        ("a single-quoted substitution is a string",
         "git -C \"%s\" stash list '$(./git re" % linked + "set --hard)'", main, ALLOW),
        ("a double-quoted substitution running no git",
         'git -C "%s" commit -m "wip $(date)"' % linked, main, ALLOW),

        # A boundary character inside a substitution separates the
        # commands in there and does not end the command outside it.
        # Leaving it in ended the span early and cleared the verb behind.
        ("a semicolon inside a substitution is not a boundary",
         "git $(echo;) re" + "set --hard", main, DENY),
        ("a pipe inside a substitution is not a boundary",
         "git $(echo|cat) re" + "set --hard", main, DENY),
        ("a substitution opening before the verb and closing after it",
         "echo $( ; git re" + "set --hard )", main, DENY),

        # The configuration spellings of the option the guard refuses by
        # name. git-config(1) documents core.worktree as the setting
        # --work-tree writes, so refusing one and clearing the other
        # turned the guard's own principle on which spelling was reached
        # for.
        ("core.worktree set on the command line",
         'git -C "%s" -c core.worktree=%s re' % (linked, main) + 'set --hard',
         main, DENY),
        ("core.bare set on the command line",
         'git -C "%s" -c core.bare=false commit -m wip' % linked, main, DENY),
        ("an unrelated configuration override still passes",
         'git -C "%s" -c core.pager=cat stash pop' % linked, main, ALLOW),

        # What counting invocations with the finder's own pattern costs.
        # A path with a git component in it reads as a second invocation,
        # so an unquoted `-C` naming one is refused; quoting the path
        # blanks it for detection and the invocation passes. Refusing too
        # much is the direction this guard is allowed to be wrong in, and
        # the escape is the one the guard already prescribes elsewhere.
        ("an unquoted -C path carrying a git component reads as a second invocation",
         "git -C %s stash pop" % componented, main, DENY),
        ("quoting that path clears it",
         'git -C "%s" stash pop' % componented, main, ALLOW),

        # A command mentioning no git at all never reaches the rule.
        ("no git in the command", "rm -rf build", main, ALLOW),
    ])

    # A deny-set verb standing inside a path is not the subcommand. Every
    # verb in the set is also a word a directory can be named after, and
    # this board names them that way, because a worktree belongs to a
    # card at a stage and is written `<card>-<stage>/wt`. A regular
    # expression's `\b` ends a word at a hyphen where a shell does not,
    # so `merge` was found inside `dinah-249-merge`, `am` inside
    # `card-am` and `rm` inside `rm-fixture`, and a read-only command was
    # refused for a verb it never ran. Each command below runs a verb the
    # deny set does not carry at all, so the only way to refuse one is to
    # read the path as a subcommand.
    for name in PATH_VERBS:
        for shape, segment in (("ending a segment", "card-%s" % name),
                               ("beginning a segment", "%s-fixture" % name),
                               ("standing as a whole segment", name)):
            spelled = slashed(os.path.join(root, "scratch", segment, "wt"))
            table.append(("%s %s of a -C path is not a subcommand" % (name, shape),
                          "git -C %s rev-parse HEAD" % spelled, main, ALLOW))

    table.extend([
        # The three reads that were refused, spelled as they were found.
        # The merge-stage path here names no worktree, which is how the
        # defect was met: an agent reads through a directory a card has
        # already finished with, and the guard answers that it blocked a
        # merge. A path that does name a live worktree is the case below.
        ("a status read through a merge-stage path",
         "git -C %s status --short"
         % slashed(os.path.join(root, "scratch", "dinah-250-merge", "wt")), main, ALLOW),
        ("a status read through a live merge-stage worktree",
         "git -C %s status --short" % slashed(verbnamed), main, ALLOW),
        # What the second reading costs, stated as a case rather than
        # left to be discovered. A span carrying expansion syntax is read
        # again with no word boundaries at all, so the very path shape
        # this card cleared is refused again once a substitution stands
        # in the same span. The pair is here so that the cost stays
        # visible and so that narrowing it later reddens something.
        ("a verb-named path is refused again once the span holds a substitution",
         "git -C %s log --oneline -1 $(echo x)"
         % slashed(os.path.join(root, "scratch", "dinah-250-merge", "wt")), main, DENY),
        ("the same read without the substitution is allowed",
         "git -C %s log --oneline -1"
         % slashed(os.path.join(root, "scratch", "dinah-250-merge", "wt")), main, ALLOW),
        ("a log read through an am-named worktree",
         "git -C %s log --oneline -1"
         % slashed(os.path.join(root, "scratch", "card-am", "wt")), main, ALLOW),
        ("a diff read through an rm-named worktree",
         "git -C %s diff" % slashed(os.path.join(root, "scratch", "rm-fixture", "wt")),
         main, ALLOW),

        # The command the guard's own refusal text prescribes, refused by
        # the guard that printed it. A worktree is added from the
        # repository it belongs to, so the `-C` of a creating command
        # names the main checkout by necessity, and no rewording of the
        # naming convention could have cleared this one.
        ("the board's own merge-stage worktree can be created",
         "git -C %s worktree add --detach %s HEAD"
         % (slashed(main), slashed(os.path.join(root, "scratch", "dinah-293-merge", "wt"))),
         main, ALLOW),

        # The same defect one layer down, on the last long flag that was
        # read without asking where its word began. A file called
        # `seed--hard.txt` is not the `--hard` flag, and a soft reset is
        # in nobody's deny set.
        #
        # The verb is spelled across a concatenation here, as it is in
        # `MUTATING` and for the same reason: an agent editing this
        # repository writes the file through a shell, and the live guard
        # reads the writing command. Grep for `--soft` rather than for
        # the whole invocation when you come looking for this case.
        ("a long flag inside a filename is not that flag",
         "git -C %s re" % slashed(main) + "set --soft HEAD -- seed--hard.txt", main, ALLOW),

        # The other face of that rule. An exemption asks for a flag too,
        # so anchoring the flag makes the exemption harder to satisfy,
        # and a file named after `--cached` no longer excuses an rm that
        # really would delete it.
        ("a filename spelled like --cached does not exempt an rm",
         "git rm seed--cached.txt", main, DENY),

        # Where a word ends carries the other half of the question, and
        # these two are the cases that ask it. A path can begin with a
        # verb's name as readily as it can end with one, and a
        # subcommand that merely starts with a verb's letters is a
        # different subcommand.
        ("a relative path beginning with a verb name is not a subcommand",
         "git -C %s log -1 -- merge/notes.txt" % slashed(main), main, ALLOW),
        ("merge-base is not merge", "git merge-base HEAD origin/main", main, ALLOW),

        # The operand of the force-with-lease rule stays a regular
        # expression's word rather than a shell's. A ref is written with
        # slashes and colons, so requiring `main` to stand alone there
        # would clear a forced push at the trunk.
        ("--force-with-lease at a fully spelled trunk ref",
         "git push --force-with-lease origin refs/heads/main", main, DENY),
        ("--force-with-lease at a colon-spelled trunk ref",
         "git push --force-with-lease origin HEAD:refs/heads/main", main, DENY),
    ])

    # The other direction, and it is the half a suite of refusals cannot
    # prove. Narrowing a verb to a whole shell word must not stop the
    # guard finding a verb that is one, so every deny-set command runs
    # again with a path carrying its own verb's name standing beside it.
    # A rule that stopped matching would clear all of these at once.
    for name, command in MUTATING:
        segment = "card-" + command.split(" ")[1]
        decorated = "%s %s" % (command, slashed(os.path.join(root, "scratch", segment, "wt")))
        table.append(("%s beside a path carrying its own verb" % name,
                      decorated, main, DENY))
        table.append(("%s -C the main checkout, beside such a path" % name,
                      qualified(decorated, main), main, DENY))
        table.append(("%s -C a worktree whose own path carries a verb" % name,
                      "git -C %s %s" % (slashed(verbnamed), command[len("git "):]),
                      main, ALLOW))

    return table


def reason_cases(main, linked):
    """Cases that read the refusal text rather than only the verdict.

    Three of these cannot be written as verdict cases at all. An
    msys-spelled path that names the main checkout, and one that names
    an ordinary directory, are both refused whether or not the guard
    understands the spelling; only the sentence the guard writes says
    which directory it judged, and only the Windows spelling in that
    sentence proves the normalisation ran. The last two read which
    remedy the refusal offered.
    """
    elsewhere = tempfile.gettempdir()
    entries = []
    if os.name == "nt":
        entries.extend([
            ("-C the main checkout, Git Bash spelling, is the main checkout",
             qualified("git stash pop", msys(main)), linked,
             ["-C %s is the main checkout" % windows_spelled(main)], []),
            ("worktree removal -C the main checkout, Git Bash spelling",
             qualified("git worktree remove old", msys(main)), linked,
             ["-C %s is the main checkout" % windows_spelled(main)], []),
            ("-C a real directory that is no worktree, Git Bash spelling",
             qualified("git clean -fdx", msys(elsewhere)), main,
             ["-C %s is not a git worktree" % windows_spelled(elsewhere)], []),
        ])
    entries.extend([
        ("-C . is refused for being relative",
         "git -C . stash pop", linked,
         ["-C . is relative, so it names no directory on its own"], []),
        ("-C a relative subdirectory is refused for being relative",
         "git -C scratch/card-impl/wt stash pop", main,
         ["is relative, so it names no directory on its own"], []),
        ("-C /cool/path is refused for naming no worktree",
         "git -C /cool/path clean -fdx", main,
         ["-C /cool/path is not a git worktree"], []),
        ("a trailing -C is refused for naming nothing",
         'git -C "%s" commit -m wip -C' % linked, main,
         ["-C names no directory"], []),
        ("a refused removal is told to read the path git knows",
         qualified("git worktree remove old", main), linked,
         ["worktree list"], ["Create one with"]),
        ("every other refused verb keeps the remedy that makes a worktree",
         RESET, main,
         ["Create one with"], []),
    ])
    return entries


def spelling_checks():
    """`windows_drive_spelling`, both branches, on whichever host runs this.

    This is weaker than the cases above in one respect and the weakness
    is worth naming: it proves the function's branches, not that
    `fault` passes the right `on_windows` value on a host this
    repository cannot run.
    """
    spell = load_guard().windows_drive_spelling
    return [
        ("a drive mount becomes a Windows path with an upper-case letter",
         spell("/c/dinah-scratch/x", on_windows=True), "C:/dinah-scratch/x"),
        ("an already upper-case drive mount is spelled the same way",
         spell("/C/dinah-scratch/x", on_windows=True), "C:/dinah-scratch/x"),
        ("off Windows a drive mount is an ordinary path and stays one",
         spell("/c/dinah-scratch/x", on_windows=False), "/c/dinah-scratch/x"),
        ("a POSIX path that is not a drive mount is untouched",
         spell("/cool/path", on_windows=True), "/cool/path"),
        ("a relative path is untouched",
         spell("scratch/card-impl/wt", on_windows=True), "scratch/card-impl/wt"),
    ]


def main():
    root = tempfile.mkdtemp(prefix="deny-destructive-hook-")
    failures = 0
    total = 0
    try:
        checkout, linked, spaced, nested, componented, verbnamed = build_repo(root)
        for name, command, cwd, want in cases(
                root, checkout, linked, spaced, nested, componented, verbnamed):
            total += 1
            try:
                got = verdict(command, cwd)
            except AssertionError as err:
                got = "error: %s" % err
            mark = "ok  " if got == want else "FAIL"
            if got != want:
                failures += 1
            print("%s %-58s want=%-5s got=%s" % (mark, name, want, got))

        for name, command, cwd, needs, forbids in reason_cases(checkout, linked):
            total += 1
            try:
                text = hook_reason(command, cwd)
                if text is None:
                    detail = "allowed, so it printed no reason"
                else:
                    missing = [want for want in needs if want not in text]
                    present = [away for away in forbids if away in text]
                    detail = "ok" if not missing and not present else (
                        "missing=%r unwanted=%r" % (missing, present))
            except AssertionError as err:
                detail = "error: %s" % err
            mark = "ok  " if detail == "ok" else "FAIL"
            if detail != "ok":
                failures += 1
            print("%s %-58s reason: %s" % (mark, name, detail))

        for name, got, want in spelling_checks():
            total += 1
            mark = "ok  " if got == want else "FAIL"
            if got != want:
                failures += 1
            print("%s %-58s want=%s got=%s" % (mark, name, want, got))
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
