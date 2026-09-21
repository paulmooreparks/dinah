#!/usr/bin/env python3
"""Build the starved directory the author of the interchange reader works in.

The independent reader under conformance/interchange-reader/ is written by an
author whose file tools are confined to one directory holding two files: the
published profile and the brief. This script builds that directory from git at
one recorded commit, checks that it holds exactly those two files and an empty
out/ directory, and writes a manifest naming the commit and each file's hash so
a reviewer can check what the author was given.

    python scripts/interchange_reader_prepare.py --repo <worktree>
        --ref <commit-ish> --into <starved directory>
        --evidence <evidence directory>

Exit status is 0 when the directory and the manifest were written, 1 when the
target is refused or the built directory is not in the expected state, and 2
on a usage error. A target that is not empty is refused before anything is
written to it. Any later refusal removes the target directory.
"""

import argparse
import hashlib
import json
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

# The two files the author may read, as (name in the starved directory, path
# in the repository). Nothing else is copied.
GIVEN = (
    ("core-profile.md", "docs/spec/core-profile.md"),
    ("BRIEF.md", "conformance/interchange-reader/BRIEF.md"),
)

EXPECTED_ENTRIES = {"core-profile.md", "BRIEF.md", "out"}


class Refused(Exception):
    """The target cannot be prepared, and the message says why."""


def git(repo, *args):
    """Run git in repo and return its standard output as bytes."""
    done = subprocess.run(
        ["git", "-C", str(repo), *args], capture_output=True, check=False
    )
    if done.returncode != 0:
        raise Refused(
            "git %s failed: %s"
            % (" ".join(args), done.stderr.decode("utf-8", "replace").strip())
        )
    return done.stdout


def inside_a_repository(directory):
    """Report whether git sees directory as part of a working tree."""
    done = subprocess.run(
        ["git", "-C", str(directory), "rev-parse", "--is-inside-work-tree"],
        capture_output=True,
        check=False,
    )
    return done.returncode == 0


def check_contents(into):
    """Raise Refused unless into holds exactly the expected entries.

    The entries must be the two given files and an empty out/ directory, and
    none of them may be a symbolic link or a junction.
    """
    entries = {entry.name: entry for entry in into.iterdir()}
    if set(entries) != EXPECTED_ENTRIES:
        raise Refused(
            "%s holds %s, expected exactly %s"
            % (into, sorted(entries), sorted(EXPECTED_ENTRIES))
        )
    for entry in entries.values():
        if entry.is_symlink() or entry.is_junction():
            raise Refused("%s is a link or a junction" % entry)
    out = entries["out"]
    if not out.is_dir():
        raise Refused("%s is not a directory" % out)
    held = list(out.iterdir())
    if held:
        raise Refused("%s is not empty: %s" % (out, sorted(p.name for p in held)))
    for name in ("core-profile.md", "BRIEF.md"):
        if not entries[name].is_file():
            raise Refused("%s is not a regular file" % entries[name])


def prepare(repo, ref, into, evidence, after_populate=None):
    """Build the starved directory and write the manifest.

    after_populate, when given, is called with the target after the files are
    written and before the contents check, which is how the tests plant an
    extra entry.
    """
    into = Path(into).resolve()
    evidence = Path(evidence).resolve()
    if into.exists() and (not into.is_dir() or any(into.iterdir())):
        raise Refused("%s exists and is not empty" % into)
    into.mkdir(parents=True, exist_ok=True)
    try:
        if inside_a_repository(into):
            raise Refused("%s sits inside a git working tree" % into)
        commit = git(repo, "rev-parse", ref + "^{commit}").decode("ascii").strip()
        files = []
        for name, source in GIVEN:
            data = git(repo, "show", "%s:%s" % (commit, source))
            (into / name).write_bytes(data)
            files.append(
                {
                    "path": name,
                    "bytes": len(data),
                    "sha256": hashlib.sha256(data).hexdigest(),
                }
            )
        (into / "out").mkdir()
        if after_populate is not None:
            after_populate(into)
        check_contents(into)
    except Exception:
        shutil.rmtree(into, ignore_errors=True)
        raise
    manifest = {
        "commit": commit,
        "ref": ref,
        "created": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "into": str(into),
        "files": files,
    }
    evidence.mkdir(parents=True, exist_ok=True)
    (evidence / "manifest.json").write_text(
        json.dumps(manifest, indent=2) + "\n", encoding="utf-8", newline="\n"
    )
    return manifest


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Build the starved directory for the interchange reader."
    )
    parser.add_argument("--repo", required=True)
    parser.add_argument("--ref", required=True)
    parser.add_argument("--into", required=True)
    parser.add_argument("--evidence", required=True)
    args = parser.parse_args(argv)
    try:
        manifest = prepare(args.repo, args.ref, args.into, args.evidence)
    except Refused as refusal:
        print("refused: %s" % refusal, file=sys.stderr)
        return 1
    print(json.dumps(manifest, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
