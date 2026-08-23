#!/usr/bin/env python3
"""Capture a dinah-core-0.4 fixture by replaying populate.txt through the built
binary, then update the manifest digest."""
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
TARGET = COMPAT_DIR / "dinah-core-0.4"
BINARY = Path(os.environ.get("TEMP", "/tmp")) / "dinah-capture.exe"


def main() -> int:
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

        if TARGET.exists():
            shutil.rmtree(TARGET)
        # The populate sequence creates the workbench under .dinah/{id}/, but
        # the compat fixture stores the workbench at its own root, so copy the
        # inner directory up rather than the wrapper.
        user_base = root / ".dinah"
        ids = [p for p in user_base.iterdir() if p.is_dir()] if user_base.exists() else []
        if len(ids) == 1:
            source = ids[1 - 1]
        else:
            source = root
        shutil.copytree(source, TARGET)

        digest = digest_tree(TARGET, "dinah-core-0.4")
        update_manifest(COMPAT_DIR, "dinah-core-0.4", digest)
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
    path = compat_dir / "manifest.json"
    data = json.loads(path.read_text())
    for row in data["fixtures"]:
        if row["directory"] == directory:
            row["digest"] = digest
            break
    else:
        print(f"directory {directory} not in manifest", file=sys.stderr)
        sys.exit(1)
    path.write_text(json.dumps(data, indent=2) + "\n")


if __name__ == "__main__":
    sys.exit(main())