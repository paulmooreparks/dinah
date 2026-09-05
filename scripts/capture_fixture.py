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
import re
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
COMPAT_TEST = WD / "cmd" / "dinah" / "compat_test.go"
BINARY = Path(os.environ.get("DINAH_CAPTURE_BINARY") or Path(os.environ.get("TEMP", "/tmp")) / "dinah-capture.exe")


def populate_inputs() -> list:
    """Return the files populate.txt names as arguments, read off the test.

    The list used to be written out here as well as in cmd/dinah/compat_test.go,
    and a card adding a file to the sequence had to remember both. It did not,
    and the capture failed on the line naming the file it had not copied. The
    test is the declaration, so the capture reads it rather than keeping a
    second copy that can fall behind.
    """
    body = COMPAT_TEST.read_text(encoding="utf-8")
    start = body.index("var populateInputs = []string{")
    end = body.index("}", start)
    return re.findall(r'"([^"]+)"', body[start:end])


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

        for name in populate_inputs():
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
            if stripped.startswith("hand-edit "):
                hand_edit(stripped[len("hand-edit "):], root, env)
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



def hand_edit(step: str, root: Path, env: dict) -> None:
    """Rewrite a card's column in its anchor and leave its journal alone.

    This is the one step of the population sequence no command performs, and
    the sequence needs it because manual_correction is the tool's answer to
    exactly this edit. The step reads `hand-edit <card> column <slug>`, and
    both references are resolved by asking the binary rather than by parsing
    the tree here, so this script and cmd/dinah's own replay stay one
    implementation apart rather than two.
    """
    fields = step.split()
    if len(fields) != 3 or fields[1] != "column":
        raise SystemExit(f"a hand-edit step reads `hand-edit <card> column <slug>`, got {step!r}")
    anchor = run_capture([str(BINARY), "path", fields[0]], root, env)
    listed = run_capture([str(BINARY), "--json", "columns"], root, env)
    wanted = next((c["id"] for c in json.loads(listed) if c["slug"] == fields[2]), None)
    if wanted is None:
        raise SystemExit(f"the workbench declares no column with the slug {fields[2]!r}")
    path = Path(anchor.strip())
    lines = path.read_text(encoding="utf-8").split("\n")
    for i, line in enumerate(lines):
        if line.startswith("column: "):
            lines[i] = "column: " + wanted
            path.write_text("\n".join(lines), encoding="utf-8", newline="")
            return
    raise SystemExit(f"the anchor at {path} carries no column key to edit")


def run_capture(argv: list, root: Path, env: dict) -> str:
    """Run one command of the tool and answer with its stdout."""
    result = subprocess.run(argv, cwd=str(root), env=env, capture_output=True, text=True)
    if result.returncode != 0:
        raise SystemExit(f"{argv[1:]}: exit {result.returncode}\n{result.stdout}\n{result.stderr}")
    return result.stdout


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
    # The newline is fixed rather than the platform's, so a capture run on
    # Windows does not rewrite the whole manifest with carriage returns.
    path.write_text(json.dumps(data, indent=2) + "\n", newline="")


if __name__ == "__main__":
    sys.exit(main())