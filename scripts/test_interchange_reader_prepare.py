"""Tests of scripts/interchange_reader_prepare.py.

Run with python -m unittest discover -s scripts -p "test_interchange_reader_*.py" -v.
Each case builds a small repository of its own carrying the files the script
copies, so no case reads this repository's history. The update cases commit a
second revision of the profile that adds, retires and rewords a statement and
bumps the version identity, with a provenance.json naming the first revision.
"""

import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import interchange_reader_compare as compare  # noqa: E402
import interchange_reader_prepare as prepare  # noqa: E402

PROFILE = b"# A profile\n\nVersion identity: `x 0.1`\n"
BRIEF = b"# A brief\n"
UPDATE_BRIEF = b"# An update brief\n"

PROFILE_V1 = (
    b"# A profile\n\nVersion identity: `x 0.1`, maturity channel `dev`.\n\n"
    b"[CORE-A-1] A tool MUST keep the first rule.\n"
    b"[CORE-A-2] A tool MUST keep the second rule.\n"
    b"[CORE-B-1] A tool MAY do the old thing.\n"
)
PROFILE_V2 = (
    b"# A profile\n\nVersion identity: `x 0.2`, maturity channel `dev`.\n\n"
    b"[CORE-A-1] A tool MUST keep the first rule.\n"
    b"[CORE-A-2] A tool MUST keep the second rule, as reworded.\n"
    b"[CORE-C-1] A tool MAY do the new thing.\n"
)

SEED = {
    "reader.py": b"print('reader')\n",
    "scope.json": b'{"profile": "x 0.1", "statements": {}}\n',
    "READINGS.md": b"# Readings\n",
    "fixtures/accept-one.json": b"{}\n",
    "fixtures/expectations.json": b'{"fixtures": []}\n',
    "tests/__init__.py": b"",
    "tests/test_reader.py": b"import unittest\n",
}


def git(directory, *args):
    return subprocess.run(
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
    ).stdout.decode("ascii").strip()


def sha256(data):
    return hashlib.sha256(data).hexdigest()


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
        self.assertEqual(written["mode"], "construction")
        self.assertEqual(
            sorted(written),
            ["commit", "created", "files", "into", "mode", "ref"],
        )
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


class UpdateTest(unittest.TestCase):
    """The update run, against a repository of two profile revisions."""

    def setUp(self):
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.scratch = Path(scratch.name)
        self.repo = self.scratch / "repo"
        self.reader = self.repo / "conformance" / "interchange-reader"
        (self.repo / "docs" / "spec").mkdir(parents=True)
        self.reader.mkdir(parents=True)
        self.profile = self.repo / "docs" / "spec" / "core-profile.md"
        self.profile.write_bytes(PROFILE_V1)
        (self.reader / "BRIEF.md").write_bytes(BRIEF)
        (self.reader / "UPDATE-BRIEF.md").write_bytes(UPDATE_BRIEF)
        (self.reader / "README.md").write_bytes(b"# Readme\n")
        for name, data in SEED.items():
            path = self.reader / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
        git(self.repo, "init", "-q")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-q", "-m", "first")
        self.first = git(self.repo, "rev-parse", "HEAD")
        self.write_provenance(self.first, sha256(PROFILE_V1))
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-q", "-m", "provenance")
        self.profile.write_bytes(PROFILE_V2)
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-q", "-m", "second")
        self.second = git(self.repo, "rev-parse", "HEAD")
        self.evidence = self.scratch / "evidence"
        self.into = self.scratch / "starved"

    def write_provenance(self, commit, profile_hash):
        (self.reader / "provenance.json").write_bytes(
            json.dumps({"commit": commit, "profile_sha256": profile_hash}).encode("utf-8")
        )

    def commit_provenance(self, commit, profile_hash, message="reprovenance"):
        self.write_provenance(commit, profile_hash)
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-q", "-m", message)
        return git(self.repo, "rev-parse", "HEAD")

    def run_update(self, ref="HEAD", **extra):
        return prepare.prepare(
            str(self.repo), ref, str(self.into), str(self.evidence), update=True, **extra
        )

    def expected_diff(self, base, commit):
        return subprocess.run(
            [
                "git", "-C", str(self.repo), "diff", "--no-color", "--no-ext-diff",
                "--unified=3", base, commit, "--", "docs/spec/core-profile.md",
            ],
            check=True,
            capture_output=True,
        ).stdout

    def test_an_update_target_holds_the_five_entries_and_a_seeded_out(self):
        manifest = self.run_update()
        self.assertEqual(
            sorted(p.name for p in self.into.iterdir()),
            ["BRIEF.md", "UPDATE-BRIEF.md", "core-profile.diff", "core-profile.md", "out"],
        )
        self.assertEqual((self.into / "core-profile.md").read_bytes(), PROFILE_V2)
        self.assertEqual((self.into / "BRIEF.md").read_bytes(), BRIEF)
        self.assertEqual((self.into / "UPDATE-BRIEF.md").read_bytes(), UPDATE_BRIEF)
        diff = self.expected_diff(self.first, self.second)
        self.assertEqual((self.into / "core-profile.diff").read_bytes(), diff)
        self.assertTrue(diff.startswith(b"diff --git a/docs/spec/core-profile.md b/docs/spec/core-profile.md"))
        seeded = sorted(
            p.relative_to(self.into / "out").as_posix()
            for p in (self.into / "out").rglob("*")
            if p.is_file()
        )
        self.assertEqual(seeded, sorted(SEED))
        for name, data in SEED.items():
            self.assertEqual((self.into / "out" / name).read_bytes(), data)
        written = json.loads((self.evidence / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(written, manifest)
        self.assertEqual(written["mode"], "update")
        self.assertEqual(written["commit"], self.second)
        self.assertEqual(written["base_commit"], self.first)
        self.assertEqual(written["base_resolved_by"], "provenance")
        self.assertNotIn("base_search_examined", written)
        self.assertEqual(written["version_identity"], {"base": "x 0.1", "ref": "x 0.2"})
        self.assertEqual(
            written["statements"],
            {"added": ["CORE-C-1"], "removed": ["CORE-B-1"], "reworded": ["CORE-A-2"]},
        )
        self.assertEqual(
            [entry["path"] for entry in written["files"]],
            sorted(
                ["core-profile.md", "core-profile.diff", "BRIEF.md", "UPDATE-BRIEF.md"]
                + ["out/" + name for name in SEED]
            ),
        )
        by_path = {entry["path"]: entry for entry in written["files"]}
        self.assertEqual(
            by_path["out/reader.py"],
            {
                "path": "out/reader.py",
                "bytes": len(SEED["reader.py"]),
                "sha256": sha256(SEED["reader.py"]),
                "source": "conformance/interchange-reader/reader.py",
            },
        )
        self.assertEqual(by_path["core-profile.md"]["source"], "docs/spec/core-profile.md")
        self.assertEqual(by_path["core-profile.diff"]["sha256"], sha256(diff))
        self.assertEqual(
            by_path["core-profile.diff"]["source"],
            "git diff %s..%s -- docs/spec/core-profile.md" % (self.first, self.second),
        )
        self.assertEqual(by_path["UPDATE-BRIEF.md"]["source"], "conformance/interchange-reader/UPDATE-BRIEF.md")

    def test_the_base_resolves_by_option(self):
        manifest = self.run_update(base=self.first)
        self.assertEqual(manifest["base_commit"], self.first)
        self.assertEqual(manifest["base_resolved_by"], "option")
        self.assertNotIn("base_search_examined", manifest)

    def test_the_base_resolves_by_search_when_the_recorded_commit_is_gone(self):
        self.commit_provenance("0" * 40, sha256(PROFILE_V1))
        head = git(self.repo, "rev-parse", "HEAD")
        manifest = self.run_update()
        self.assertEqual(manifest["commit"], head)
        self.assertEqual(manifest["base_commit"], self.first)
        self.assertEqual(manifest["base_resolved_by"], "search")
        # rev-list over the profile lists the second commit and then the
        # first, so the first is found on the second commit examined.
        self.assertEqual(manifest["base_search_examined"], 2)

    def test_the_base_resolves_by_search_when_the_recorded_commit_carries_another_profile(self):
        self.commit_provenance(self.second, sha256(PROFILE_V1))
        manifest = self.run_update()
        self.assertEqual(manifest["base_commit"], self.first)
        self.assertEqual(manifest["base_resolved_by"], "search")

    def test_a_base_option_whose_profile_hash_mismatches_is_refused(self):
        with self.assertRaisesRegex(prepare.Refused, "hashes to .* and provenance.json records"):
            self.run_update(base=self.second)
        self.assertFalse(self.into.exists())
        self.assertFalse((self.evidence / "manifest.json").exists())

    def test_a_hash_no_commit_carries_is_refused(self):
        self.commit_provenance("0" * 40, "f" * 64)
        with self.assertRaisesRegex(prepare.Refused, "no commit on the history of .* carries"):
            self.run_update()
        self.assertFalse(self.into.exists())

    def test_an_empty_diff_is_refused(self):
        self.commit_provenance(self.second, sha256(PROFILE_V2))
        with self.assertRaisesRegex(prepare.Refused, "the profile has not changed since"):
            self.run_update()
        self.assertFalse(self.into.exists())

    def test_base_without_update_is_a_usage_error(self):
        with self.assertRaises(SystemExit) as stopped:
            prepare.main(
                [
                    "--repo", str(self.repo), "--ref", "HEAD", "--into", str(self.into),
                    "--evidence", str(self.evidence), "--base", self.first,
                ]
            )
        self.assertEqual(stopped.exception.code, 2)
        with self.assertRaisesRegex(prepare.Refused, "only with --update"):
            prepare.prepare(str(self.repo), "HEAD", str(self.into), str(self.evidence), base=self.first)
        self.assertFalse(self.into.exists())

    def test_a_missing_provenance_is_refused(self):
        (self.reader / "provenance.json").unlink()
        git(self.repo, "add", "-A", ".")
        git(self.repo, "commit", "-q", "-m", "no provenance")
        with self.assertRaisesRegex(prepare.Refused, "provenance.json is missing"):
            self.run_update()
        self.assertFalse(self.into.exists())

    def test_a_link_planted_under_out_is_refused_and_the_target_removed(self):
        elsewhere = self.scratch / "elsewhere"
        elsewhere.mkdir()

        def plant(target):
            try:
                (target / "out" / "fixtures" / "link").symlink_to(elsewhere, target_is_directory=True)
            except OSError as err:
                self.skipTest("this machine cannot create a symbolic link: %s" % err)

        with self.assertRaisesRegex(prepare.Refused, "a link or a junction"):
            self.run_update(after_populate=plant)
        self.assertFalse(self.into.exists())
        self.assertTrue(elsewhere.exists())

    def test_an_extra_top_level_entry_is_refused(self):
        def plant(target):
            (target / "notes.txt").write_text("x", encoding="utf-8")

        with self.assertRaisesRegex(prepare.Refused, "expected exactly"):
            self.run_update(after_populate=plant)
        self.assertFalse(self.into.exists())

    def test_the_seed_excludes_exactly_the_project_owned_names(self):
        (self.reader / "NOTES.md").write_bytes(b"notes\n")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-q", "-m", "notes")
        manifest = self.run_update()
        self.assertIn("out/NOTES.md", [entry["path"] for entry in manifest["files"]])
        self.assertEqual((self.into / "out" / "NOTES.md").read_bytes(), b"notes\n")
        for name in ("BRIEF.md", "UPDATE-BRIEF.md", "README.md", "provenance.json"):
            self.assertFalse((self.into / "out" / name).exists(), name)
        # Patch the compare script's set in place: a prepare script that
        # restated the names would not see the patch and would still seed
        # NOTES.md.
        compare.NOT_THE_AUTHORS.add("NOTES.md")
        self.addCleanup(compare.NOT_THE_AUTHORS.discard, "NOTES.md")
        self.into = self.scratch / "starved-2"
        manifest = self.run_update()
        self.assertNotIn("out/NOTES.md", [entry["path"] for entry in manifest["files"]])
        self.assertFalse((self.into / "out" / "NOTES.md").exists())

    def test_a_seed_without_the_reader_is_refused(self):
        (self.reader / "reader.py").unlink()
        git(self.repo, "add", "-A", ".")
        git(self.repo, "commit", "-q", "-m", "no reader")
        with self.assertRaisesRegex(prepare.Refused, "lacks reader.py"):
            self.run_update()
        self.assertFalse(self.into.exists())

    def test_the_command_line_runs_an_update(self):
        status = prepare.main(
            [
                "--repo", str(self.repo), "--ref", "HEAD", "--into", str(self.into),
                "--evidence", str(self.evidence), "--update", "--base", self.first,
            ]
        )
        self.assertEqual(status, 0)
        written = json.loads((self.evidence / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(written["base_resolved_by"], "option")


if __name__ == "__main__":
    unittest.main()
