#!/usr/bin/env python3
r"""Every hook in .claude/settings.json names its script by an absolute path.

Run it from anywhere: `python scripts/hooks/test-hook-registration-cwd-independence.py`.
It prints every case either way and exits non-zero when any of them fails, so
a failure names itself rather than needing a debugger.

The defect this pins bricked a session rather than failing one command. A hook
registered as `python scripts/hooks/deny-destructive-git.py` resolves that path
against the shell's working directory, so the moment a shell stood anywhere but
the repository root the interpreter could not open the guard, and every
subsequent Bash and PowerShell call was refused with `can't open file`. The
guard runs before the command, so even a `cd` back to the root was refused and
the session could not walk itself out.

The two suites beside this one would not have caught that. Both invoke the
guard at `os.path.join(os.path.dirname(os.path.abspath(__file__)), ...)`, a
path built from their own location, so neither reads `.claude/settings.json`
and neither goes through a shell that expands `$CLAUDE_PROJECT_DIR`. This file
reads the registration itself.

PART 1 is static. It walks every event under the top-level `hooks` object and
asserts that each command's script path begins with `$CLAUDE_PROJECT_DIR/`. It
reads a quoted argument and a bare one alike, because the spelling it exists to
catch is the unquoted one.

PART 2 is dynamic and does not read Part 1's parse back out. It runs each
command string through a real `bash` from a temporary directory outside the
repository, with `CLAUDE_PROJECT_DIR` set, and asks whether the interpreter
could open the file. That reproduces the original failure on demand instead of
by accident, and it would still fail if Part 1's parser had a bug that let a
bad entry through.

The script locates the repository from its own path, never from os.getcwd(),
because a test whose own cwd-independence is unverified cannot vouch for
anyone else's.

Two conditions fail this run rather than skipping it: a command whose shape
this file cannot parse, and a command naming a script this file has no stdin
payload for. Either one is a hook the check does not cover, and a hook covered
by nothing is what this file exists to prevent. Adding a hook therefore means
adding its payload here, which is the cost of the guarantee.

An optional argument points the run at a different settings file, which is how
the check is armed: copy the real one, spell a command the old relative way,
and watch both parts name it.
"""

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
SETTINGS = os.path.join(REPO_ROOT, ".claude", "settings.json")

# `python` and a single argument, quoted or bare. Both registered commands
# quote theirs, and the bare alternative is not decoration: the spelling this
# file exists to catch is the unquoted `python scripts/hooks/...`, so a pattern
# that only accepts a quoted argument would report a real regression as a shape
# it cannot parse and would never put it to a shell.
COMMAND_SHAPE = re.compile(r'\s*python\s+(?:"([^"]+)"|(\S+))\s*\Z')

REQUIRED_PREFIX = "$CLAUDE_PROJECT_DIR/"

# A minimal payload per script, chosen so the run stays hermetic. The verb
# `status` is not in the guard's deny set, so the guard answers without running
# git itself. The token hook documents that it exits 0 having done nothing when
# either environment variable is missing, and UNSET below is what holds it to
# that path.
PAYLOADS = {
    "deny-destructive-git.py": {"tool_input": {"command": "git status"}},
    "andoneer-session-tokens.py": {"transcript_path": "", "session_id": "test"},
}

UNSET = ("ANDONEER_URL", "ANDONEER_TOKEN")

# The literal text a Python interpreter prints when it is handed a script path
# it cannot open, in both phrasings.
PATH_FAILURES = ("can't open file", "No such file or directory")


def registered_commands(settings_path):
    """Every hooks.<Event>[i].hooks[j].command in the file, with its location."""
    with open(settings_path, encoding="utf-8") as handle:
        settings = json.load(handle)
    found = []
    for event, entries in sorted((settings.get("hooks") or {}).items()):
        for outer, entry in enumerate(entries):
            for inner, hook in enumerate(entry.get("hooks") or []):
                where = "%s[%d].hooks[%d]" % (event, outer, inner)
                found.append((where, hook.get("command") or ""))
    return found


def outside_repository(directory):
    """True when `directory` shares no ancestor with the repository root."""
    try:
        shared = os.path.commonpath([os.path.abspath(directory), REPO_ROOT])
    except ValueError:
        # Different drives on Windows, which is as outside as it gets.
        return True
    return os.path.normcase(shared) != os.path.normcase(REPO_ROOT)


def invoke(bash, command, cwd, payload):
    environment = dict(os.environ)
    environment["CLAUDE_PROJECT_DIR"] = REPO_ROOT
    for name in UNSET:
        environment.pop(name, None)
    return subprocess.run(
        [bash, "-c", command],
        cwd=cwd,
        env=environment,
        input=json.dumps(payload),
        capture_output=True,
        text=True,
    )


def main():
    settings_path = sys.argv[1] if len(sys.argv) > 1 else SETTINGS
    print("settings: %s" % settings_path)
    print("repository: %s" % REPO_ROOT)
    print()

    bash = shutil.which("bash")
    if not bash:
        print("no bash on PATH: part 2 needs a real shell to run a hook command")
        return 1

    commands = registered_commands(settings_path)
    if not commands:
        print("no hook command is registered at all, so this file checked nothing")
        return 1

    failures = 0
    total = 0
    runnable = []

    print("part 1: every registered command names its script absolutely")
    for where, command in commands:
        total += 1
        shape = COMMAND_SHAPE.match(command)
        if not shape:
            failures += 1
            print("FAIL %-28s this file cannot parse the shape of %r, so it "
                  "checks nothing about that hook" % (where, command))
            continue
        path = shape.group(1) or shape.group(2)
        # Part 2 runs whatever parsed, whether or not Part 1 approved of it.
        # A command Part 1 rejects is exactly the one worth putting to a real
        # shell, and the two parts are only independent if neither takes the
        # other's verdict as input.
        runnable.append((where, command, os.path.basename(path)))
        if not path.startswith(REQUIRED_PREFIX):
            failures += 1
            print("FAIL %-28s %r names its script as %r, which does not start "
                  "with %r, so the hook resolves against the shell's working "
                  "directory" % (where, command, path, REQUIRED_PREFIX))
            continue
        print("ok   %-28s %s" % (where, path))

    print()
    print("part 2: every registered command runs from outside the repository")

    sandbox = tempfile.mkdtemp(prefix="hook-registration-cwd-")
    try:
        if not outside_repository(sandbox):
            print("FAIL the temporary directory %s lies inside the repository, so "
                  "running a hook from it would prove nothing" % sandbox)
            return 1

        for where, command, script in runnable:
            total += 1
            payload = PAYLOADS.get(script)
            if payload is None:
                failures += 1
                print("FAIL %-28s no stdin payload here for %s, so the command "
                      "was not run" % (where, script))
                continue
            result = invoke(bash, command, sandbox, payload)
            broken = [text for text in PATH_FAILURES if text in result.stderr]
            if result.returncode != 0 or broken:
                failures += 1
                print("FAIL %-28s exit=%d stderr=%r"
                      % (where, result.returncode, result.stderr.strip()))
                continue
            print("ok   %-28s exit=0 from %s" % (where, sandbox))
    finally:
        # ignore_errors covers a Windows handle that has not closed yet.
        shutil.rmtree(sandbox, ignore_errors=True)

    print()
    if failures:
        print("%d of %d check(s) failed" % (failures, total))
        return 1
    print("all %d checks passed" % total)
    return 0


if __name__ == "__main__":
    sys.exit(main())
