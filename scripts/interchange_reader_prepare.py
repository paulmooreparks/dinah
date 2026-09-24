#!/usr/bin/env python3
"""Build the starved directory the author of the interchange reader works in.

The independent reader under conformance/interchange-reader/ is written and
updated by an author whose file tools are confined to one directory. This
script builds that directory from git at one recorded commit, checks that it
holds exactly what the run's brief allows, and writes a manifest naming the
commit and each file's hash so a reviewer can check what the author was given.

A construction run gives the author two files, the published profile and the
brief, beside an empty out/ directory, and the author writes the whole reader:

    python scripts/interchange_reader_prepare.py --repo <worktree>
        --ref <commit-ish> --into <starved directory>
        --evidence <evidence directory>

An update run gives the author the revised profile, a diff of the profile
since the revision the reader was last written from, both briefs, and an out/
seeded with every file the author owns at --ref, and the author changes only
what the diff requires:

    python scripts/interchange_reader_prepare.py --repo <worktree>
        --ref <commit-ish> --into <starved directory>
        --evidence <evidence directory> --update [--base <commit-ish>]

The base of an update is the revision whose profile the reader was last
written from. provenance.json at --ref records that revision's commit and the
SHA-256 of the profile it carried, and the hash is what decides, because the
commit can be unreachable after a squash merge while a commit carrying the
same blob is on the trunk. The base is taken from --base when given, else from
the recorded commit when it resolves, else by searching the history of --ref
for the newest commit whose profile carries the recorded hash; every step
hashes the blob it reads and a mismatch is refused. The set of files the seed
excludes is NOT_THE_AUTHORS, imported from the compare script rather than
restated, so the list of project-owned files lives in one place.

Exit status is 0 when the directory and the manifest were written, 1 when the
target is refused or the built directory is not in the expected state, and 2
on a usage error. A target that is not empty is refused before anything is
written to it. Any later refusal removes the target directory.
"""

import argparse
import hashlib
import json
import re
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

from interchange_reader_compare import NOT_THE_AUTHORS

ROOT = Path(__file__).resolve().parents[1]

PROFILE_PATH = "docs/spec/core-profile.md"
READER_DIR = "conformance/interchange-reader"
PROVENANCE_PATH = READER_DIR + "/provenance.json"

# The files a construction author may read, as (name in the starved directory,
# path in the repository). Nothing else is copied.
GIVEN = (
    ("core-profile.md", PROFILE_PATH),
    ("BRIEF.md", READER_DIR + "/BRIEF.md"),
)

# The files an update author is given beside out/, in the same shape.
GIVEN_FOR_UPDATE = GIVEN + (("UPDATE-BRIEF.md", READER_DIR + "/UPDATE-BRIEF.md"),)

EXPECTED_ENTRIES = {"core-profile.md", "BRIEF.md", "out"}
EXPECTED_UPDATE_ENTRIES = {
    "core-profile.md",
    "core-profile.diff",
    "BRIEF.md",
    "UPDATE-BRIEF.md",
    "out",
}

# A seed without these two files is not the reader, whatever else it holds.
SEED_MUST_HOLD = ("reader.py", "scope.json")

# The shape section 3.2 of the profile gives a normative statement, applied
# line by line, and the version identity the reader's R-1 reads from the
# profile's opening lines.
STATEMENT = re.compile(r"^\[([A-Z][A-Z0-9]*(-[A-Z0-9]+)*)\] (.+)$")
VERSION_IDENTITY = re.compile(r"Version identity: `([^`]+)`")


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


def git_succeeds(repo, *args):
    """Report whether git exits 0 in repo, without raising."""
    done = subprocess.run(
        ["git", "-C", str(repo), *args], capture_output=True, check=False
    )
    return done.returncode == 0


def inside_a_repository(directory):
    """Report whether git sees directory as part of a working tree."""
    done = subprocess.run(
        ["git", "-C", str(directory), "rev-parse", "--is-inside-work-tree"],
        capture_output=True,
        check=False,
    )
    return done.returncode == 0


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def profile_at(repo, commit):
    """Return the profile's bytes at commit, or None when the commit lacks it."""
    done = subprocess.run(
        ["git", "-C", str(repo), "show", "%s:%s" % (commit, PROFILE_PATH)],
        capture_output=True,
        check=False,
    )
    return done.stdout if done.returncode == 0 else None


def statements_of(text):
    """Return the (identifier, statement text) pairs of a profile, in order."""
    found = []
    for line in text.splitlines():
        match = STATEMENT.match(line)
        if match:
            found.append((match.group(1), match.group(3)))
    return found


def version_identity_of(text):
    """Return the version identity the profile's first ten lines state."""
    for line in text.splitlines()[:10]:
        match = VERSION_IDENTITY.search(line)
        if match:
            return match.group(1)
    return None


def compare_statements(base_text, ref_text):
    """Return the added, removed and reworded identifiers between two profiles.

    Added identifiers are listed in the order the profile at --ref publishes
    them, removed ones in the order the base published them, and reworded
    ones in the order of --ref.
    """
    base = dict(statements_of(base_text))
    ref = dict(statements_of(ref_text))
    return {
        "added": [i for i in ref if i not in base],
        "removed": [i for i in base if i not in ref],
        "reworded": [i for i in ref if i in base and base[i] != ref[i]],
    }


def read_provenance(repo, commit):
    """Return the commit and profile hash provenance.json records at commit."""
    done = subprocess.run(
        ["git", "-C", str(repo), "show", "%s:%s" % (commit, PROVENANCE_PATH)],
        capture_output=True,
        check=False,
    )
    if done.returncode != 0:
        raise Refused("%s is missing at %s" % (PROVENANCE_PATH, commit))
    try:
        recorded = json.loads(done.stdout.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as err:
        raise Refused("%s at %s is not JSON: %s" % (PROVENANCE_PATH, commit, err))
    if not isinstance(recorded, dict):
        raise Refused("%s at %s is not a JSON object" % (PROVENANCE_PATH, commit))
    for member in ("commit", "profile_sha256"):
        if not isinstance(recorded.get(member), str) or not recorded[member]:
            raise Refused(
                "%s at %s carries no %s member" % (PROVENANCE_PATH, commit, member)
            )
    return recorded["commit"], recorded["profile_sha256"]


def resolve_base(repo, commit, base_option, recorded_commit, recorded_hash):
    """Find the revision the reader was last written from.

    Returns (base commit, how it was resolved, commits examined). The option
    wins, then the recorded commit, then a search of the history of commit
    for the newest revision whose profile carries the recorded hash. Every
    step reads and hashes the blob, so no step trusts a commit identity alone.
    """
    if base_option is not None:
        base = git(repo, "rev-parse", base_option + "^{commit}").decode("ascii").strip()
        data = profile_at(repo, base)
        if data is None:
            raise Refused("%s carries no %s" % (base, PROFILE_PATH))
        if sha256(data) != recorded_hash:
            raise Refused(
                "the profile at --base %s hashes to %s, and provenance.json records %s"
                % (base, sha256(data), recorded_hash)
            )
        return base, "option", None
    if git_succeeds(repo, "rev-parse", "--verify", "--quiet", recorded_commit + "^{commit}"):
        base = git(repo, "rev-parse", recorded_commit + "^{commit}").decode("ascii").strip()
        data = profile_at(repo, base)
        if data is not None and sha256(data) == recorded_hash:
            return base, "provenance", None
    listed = git(repo, "rev-list", commit, "--", PROFILE_PATH).decode("ascii").split()
    examined = 0
    for candidate in listed:
        examined += 1
        data = profile_at(repo, candidate)
        if data is not None and sha256(data) == recorded_hash:
            return candidate, "search", examined
    raise Refused(
        "no commit on the history of %s carries a %s whose SHA-256 is %s"
        % (commit, PROFILE_PATH, recorded_hash)
    )


def profile_diff(repo, base, commit):
    """Return the bytes of the profile's diff from base to commit."""
    return git(
        repo,
        "diff",
        "--no-color",
        "--no-ext-diff",
        "--unified=3",
        base,
        commit,
        "--",
        PROFILE_PATH,
    )


def seed_paths(repo, commit):
    """Return the reader's authored files at commit, relative to its directory."""
    listed = git(repo, "ls-tree", "-r", "--name-only", commit, READER_DIR + "/")
    prefix = READER_DIR + "/"
    relative = []
    for line in listed.decode("utf-8").splitlines():
        if not line.startswith(prefix):
            continue
        name = line[len(prefix):]
        if name in NOT_THE_AUTHORS:
            continue
        relative.append(name)
    for needed in SEED_MUST_HOLD:
        if needed not in relative:
            raise Refused(
                "the reader at %s lacks %s, so there is nothing to update" % (commit, needed)
            )
    return sorted(relative)


def no_links_under(directory):
    """Raise Refused when any entry under directory is a link or a junction."""
    for entry in [directory, *directory.rglob("*")]:
        if entry.is_symlink() or entry.is_junction():
            raise Refused("%s is a link or a junction" % entry)


def check_contents(into):
    """Raise Refused unless into holds exactly a construction run's entries.

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


def check_update_contents(into):
    """Raise Refused unless into holds exactly an update run's entries.

    The entries are the four given files and a non-empty out/, and nothing
    anywhere under into may be a symbolic link or a junction.
    """
    entries = {entry.name: entry for entry in into.iterdir()}
    if set(entries) != EXPECTED_UPDATE_ENTRIES:
        raise Refused(
            "%s holds %s, expected exactly %s"
            % (into, sorted(entries), sorted(EXPECTED_UPDATE_ENTRIES))
        )
    no_links_under(into)
    out = entries["out"]
    if not out.is_dir():
        raise Refused("%s is not a directory" % out)
    if not any(out.iterdir()):
        raise Refused("%s is empty" % out)
    for name in EXPECTED_UPDATE_ENTRIES - {"out"}:
        if not entries[name].is_file():
            raise Refused("%s is not a regular file" % entries[name])


def write_given(into, repo, commit, name, source):
    """Write one file from git into the starved directory and describe it."""
    data = git(repo, "show", "%s:%s" % (commit, source))
    target = into / name
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(data)
    return data, {"path": name, "bytes": len(data), "sha256": sha256(data)}


def populate_construction(into, repo, commit):
    """Write a construction run's files and return their manifest entries."""
    files = []
    for name, source in GIVEN:
        _, entry = write_given(into, repo, commit, name, source)
        files.append(entry)
    (into / "out").mkdir()
    return {"files": files}


def populate_update(into, repo, commit, base_option):
    """Write an update run's files and return the manifest's update members."""
    recorded_commit, recorded_hash = read_provenance(repo, commit)
    base, resolved_by, examined = resolve_base(
        repo, commit, base_option, recorded_commit, recorded_hash
    )
    files = []
    texts = {}
    for name, source in GIVEN_FOR_UPDATE:
        data, entry = write_given(into, repo, commit, name, source)
        entry["source"] = source
        files.append(entry)
        texts[name] = data
    diff = profile_diff(repo, base, commit)
    if not diff:
        raise Refused("the profile has not changed since %s" % base)
    (into / "core-profile.diff").write_bytes(diff)
    files.append(
        {
            "path": "core-profile.diff",
            "bytes": len(diff),
            "sha256": sha256(diff),
            "source": "git diff %s..%s -- %s" % (base, commit, PROFILE_PATH),
        }
    )
    for relative in seed_paths(repo, commit):
        source = READER_DIR + "/" + relative
        _, entry = write_given(into, repo, commit, "out/" + relative, source)
        entry["source"] = source
        files.append(entry)
    base_text = profile_at(repo, base).decode("utf-8", "replace")
    ref_text = texts["core-profile.md"].decode("utf-8", "replace")
    members = {
        "base_commit": base,
        "base_resolved_by": resolved_by,
    }
    if resolved_by == "search":
        members["base_search_examined"] = examined
    members["version_identity"] = {
        "base": version_identity_of(base_text),
        "ref": version_identity_of(ref_text),
    }
    members["statements"] = compare_statements(base_text, ref_text)
    members["files"] = sorted(files, key=lambda entry: entry["path"])
    return members


def prepare(repo, ref, into, evidence, after_populate=None, update=False, base=None):
    """Build the starved directory and write the manifest.

    after_populate, when given, is called with the target after the files are
    written and before the contents check, which is how the tests plant an
    extra entry. update selects the update run, and base is its --base.
    """
    if base is not None and not update:
        raise Refused("--base is meaningful only with --update")
    into = Path(into).resolve()
    evidence = Path(evidence).resolve()
    if into.exists() and (not into.is_dir() or any(into.iterdir())):
        raise Refused("%s exists and is not empty" % into)
    into.mkdir(parents=True, exist_ok=True)
    try:
        if inside_a_repository(into):
            raise Refused("%s sits inside a git working tree" % into)
        commit = git(repo, "rev-parse", ref + "^{commit}").decode("ascii").strip()
        if update:
            members = populate_update(into, repo, commit, base)
        else:
            members = populate_construction(into, repo, commit)
        if after_populate is not None:
            after_populate(into)
        if update:
            check_update_contents(into)
        else:
            check_contents(into)
    except Exception:
        shutil.rmtree(into, ignore_errors=True)
        raise
    manifest = {
        "mode": "update" if update else "construction",
        "commit": commit,
        "ref": ref,
    }
    for member in ("base_commit", "base_resolved_by", "base_search_examined"):
        if member in members:
            manifest[member] = members[member]
    manifest["created"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    manifest["into"] = str(into)
    for member in ("version_identity", "statements"):
        if member in members:
            manifest[member] = members[member]
    manifest["files"] = members["files"]
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
    parser.add_argument(
        "--update",
        action="store_true",
        help="build an update run's directory, seeded with the reader at --ref",
    )
    parser.add_argument(
        "--base",
        help="the revision the reader was last written from; needs --update",
    )
    args = parser.parse_args(argv)
    if args.base is not None and not args.update:
        parser.error("--base is meaningful only with --update")
    try:
        manifest = prepare(
            args.repo,
            args.ref,
            args.into,
            args.evidence,
            update=args.update,
            base=args.base,
        )
    except Refused as refusal:
        print("refused: %s" % refusal, file=sys.stderr)
        return 1
    print(json.dumps(manifest, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
