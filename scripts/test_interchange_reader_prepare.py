"""Tests of scripts/interchange_reader_prepare.py.

Run with python -m unittest discover -s scripts -p "test_interchange_reader_*.py" -v.
Each case builds a small repository of its own carrying the two files the
script copies, so no case reads this repository's history.
"""

import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import interchange_reader_prepare as prepare  # noqa: E402

PROFILE = b"# A profile\n\nVersion identity: `x 0.1`\n"
BRIEF = b"# A brief\n"


def git(directory, *args):
    subprocess.run(
        [
            "git",
            "-c",
            "user.name=test",
            "-c",
            "user.email=test@example.invalid",
            "-c",
            "commit.gpgsign=false",
            "-C",
            str(directory),
            *args,
        ],
        check=True,
        capture_output=True,
    )


class PrepareTest(unittest.TestCase):
    def setUp(self):
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.scratch = Path(scratch.name)
        self.repo = self.scratch / "repo"
        (self.repo / "docs" / "spec").mkdir(parents=True)
        (self.repo / "conformance" / "interchange-reader").mkdir(parents=True)
        (self.repo / "docs" / "spec" / "core-profile.md").write_bytes(PROFILE)
        (self.repo / "conformance" / "interchange-reader" / "BRIEF.md").write_bytes(
            BRIEF
        )
        git(self.repo, "init", "-q")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-q", "-m", "seed")
        self.evidence = self.scratch / "evidence"

    def run_prepare(self, into, **extra):
        return prepare.prepare(str(self.repo), "HEAD", str(into), str(self.evidence), **extra)

    def test_a_clean_target_is_built_and_described(self):
        into = self.scratch / "starved"
        manifest = self.run_prepare(into)
        self.assertEqual(
            sorted(p.name for p in into.iterdir()), ["BRIEF.md", "core-profile.md", "out"]
        )
        self.assertEqual((into / "core-profile.md").read_bytes(), PROFILE)
        self.assertEqual((into / "BRIEF.md").read_bytes(), BRIEF)
        self.assertEqual(list((into / "out").iterdir()), [])
        written = json.loads((self.evidence / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(written, manifest)
        self.assertEqual(len(written["commit"]), 40)
        self.assertEqual(
            written["files"],
            [
                {
                    "path": "core-profile.md",
                    "bytes": len(PROFILE),
                    "sha256": hashlib.sha256(PROFILE).hexdigest(),
                },
                {
                    "path": "BRIEF.md",
                    "bytes": len(BRIEF),
                    "sha256": hashlib.sha256(BRIEF).hexdigest(),
                },
            ],
        )

    def test_a_target_that_is_not_empty_is_refused_and_left_alone(self):
        into = self.scratch / "starved"
        into.mkdir()
        (into / "keep.txt").write_text("mine", encoding="utf-8")
        with self.assertRaisesRegex(prepare.Refused, "not empty"):
            self.run_prepare(into)
        self.assertEqual([p.name for p in into.iterdir()], ["keep.txt"])
        self.assertEqual(
            prepare.main(
                [
                    "--repo",
                    str(self.repo),
                    "--ref",
                    "HEAD",
                    "--into",
                    str(into),
                    "--evidence",
                    str(self.evidence),
                ]
            ),
            1,
        )

    def test_a_target_inside_a_repository_is_refused_and_removed(self):
        into = self.repo / "starved"
        with self.assertRaisesRegex(prepare.Refused, "inside a git working tree"):
            self.run_prepare(into)
        self.assertFalse(into.exists())
        self.assertFalse((self.evidence / "manifest.json").exists())

    def test_an_extra_entry_after_population_is_refused_and_removed(self):
        into = self.scratch / "starved"

        def plant(target):
            (target / "extra.md").write_text("more", encoding="utf-8")

        with self.assertRaisesRegex(prepare.Refused, "expected exactly"):
            self.run_prepare(into, after_populate=plant)
        self.assertFalse(into.exists())
        self.assertFalse((self.evidence / "manifest.json").exists())

    def test_a_link_in_place_of_a_given_entry_is_refused(self):
        into = self.scratch / "starved"
        elsewhere = self.scratch / "elsewhere"
        elsewhere.mkdir()

        def plant(target):
            (target / "out").rmdir()
            try:
                (target / "out").symlink_to(elsewhere, target_is_directory=True)
            except OSError as err:
                self.skipTest("this machine cannot create a symbolic link: %s" % err)

        with self.assertRaisesRegex(prepare.Refused, "a link or a junction"):
            self.run_prepare(into, after_populate=plant)
        self.assertFalse(into.exists())
        self.assertTrue(elsewhere.exists())

    def test_a_file_under_out_after_population_is_refused(self):
        into = self.scratch / "starved"

        def plant(target):
            (target / "out" / "reader.py").write_text("", encoding="utf-8")

        with self.assertRaisesRegex(prepare.Refused, "not empty"):
            self.run_prepare(into, after_populate=plant)
        self.assertFalse(into.exists())


if __name__ == "__main__":
    unittest.main()
