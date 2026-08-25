#!/usr/bin/env python3
"""Capture a compatibility fixture by replaying populate.txt through the built
binary, then update the manifest digest.

The fixture is named for the profile revision the binary claims rather than for
a revision written down here, so a card that raises the claim captures its own
fixture without editing this script. The build stamps one revision and the bump
alarm wants a fixture declaring exactly that revision, so reading the name off
the binary is what keeps the two from drifting.

Point DINAH_CAPTURE_BINARY at the build to replay, or leave it unset and the
script reads dinah-capture.exe out of the temporary directory."""
import hashlib
import json
import os
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

WD = Path(os.getcwd())
COMPAT_DIR = WD / "internal" / "bench" / "testdata" / "compat"
POPULATE = COMPAT_DIR / "populate.txt"
BINARY = Path(os.environ.get("DINAH_CAPTURE_BINARY") or Path(os.environ.get("TEMP", "/tmp")) / "dinah-capture.exe")


def fixture_name() -> str:
    """Return the fixture directory named by the binary's own conformance claim."""
    result = subprocess.run([str(BINARY), "version", "--json"], capture_output=True, text=True)
    if result.returncode != 0:
        print(f"version: exit {result.returncode}", file=sys.stderr)
        print(result.stderr, file=sys.stderr)
        sys.exit(1)
    profile = json.loads(result.stdout)["profile"]
    return profile.replace("/", "-")


def main() -> int:
    fixture = fixture_name()
    target = COMPAT_DIR / fixture
    with tempfile.TemporaryDirectory(prefix="dinah-capture-") as base:
        root = Path(base) / "workbench"
        home = Path(base) / "home"
        root.mkdir(parents=True, exist_ok=True)
        home.mkdir(parents=True, exist_ok=True)

        for name in ("payload-one.txt", "payload-two.txt", "payload-three.txt", "definition.json"):
            shutil.copy(COMPAT_DIR / name, root / name)

        env = {
            **os.environ,
            "DINAH_HOME": str(home),
            "DINAH_ACTOR": "sam",
            "DINAH_LANG": "",
            "DINAH_FORMAT": "",
            "DINAH_WORKBENCH": "",
        }

        with open(POPULATE) as fh:
            lines = fh.read().replace("\r\n", "\n").split("\n")

        for number, line in enumerate(lines, 1):
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            if stripped.startswith("wait "):
                time.sleep(0.4)
                continue
            result = subprocess.run(
                [str(BINARY), *shlex.split(stripped)],
                cwd=str(root),
                env=env,
                capture_output=True,
                text=True,
            )
            if result.returncode != 0:
                print(f"line {number} ({stripped!r}): exit {result.returncode}\n{result.stdout}\n{result.stderr}", file=sys.stderr)
                return 1
            time.sleep(0.01)

        if target.exists():
            shutil.rmtree(target)
        # The populate sequence creates the workbench under .dinah/{id}/, but
        # the compat fixture stores the workbench at its own root, so copy the
        # inner directory up rather than the wrapper.
        user_base = root / ".dinah"
        ids = [p for p in user_base.iterdir() if p.is_dir()] if user_base.exists() else []
        if len(ids) == 1:
            source = ids[1 - 1]
        else:
            source = root
        shutil.copytree(source, target)

        digest = digest_tree(target, fixture)
        update_manifest(COMPAT_DIR, fixture, digest)
        print(f"digest: {digest}")
        return 0


def digest_tree(root: Path, fixture: str) -> str:
    prefix = f"internal/bench/testdata/compat/{fixture}"
    paths = []
    for p in root.rglob("*"):
        if p.is_file():
            paths.append(str(p.relative_to(root)).replace(os.sep, "/"))
    paths.sort()
    h = hashlib.sha256()
    for rel in paths:
        h.update(f"{prefix}/{rel}".encode())
        h.update((root / rel).read_bytes())
    return h.hexdigest()


def update_manifest(compat_dir: Path, directory: str, digest: str) -> None:
    """Record the captured fixture's digest and make that fixture the sample.

    Exactly one fixture is the sample for the revision the build stamps, so the
    flag moves onto the row this run captured and comes off every other row. A
    revision captured for the first time gets a row of its own, appended, and
    every earlier fixture stays committed and stays digested.
    """
    path = compat_dir / "manifest.json"
    data = json.loads(path.read_text())
    row = next((r for r in data["fixtures"] if r["directory"] == directory), None)
    if row is None:
        row = {"directory": directory}
        data["fixtures"].append(row)
    row["digest"] = digest
    for other in data["fixtures"]:
        other.pop("sample", None)
    row["sample"] = True
    path.write_text(json.dumps(data, indent=2) + "\n")


if __name__ == "__main__":
    sys.exit(main())