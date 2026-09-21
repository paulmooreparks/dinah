"""Tests of scripts/interchange_reader_audit.py.

Run with python -m unittest discover -s scripts -p "test_interchange_reader_*.py" -v.
Each case writes a transcript of its own, one stream-json line per tool call,
nested the way the script must not rely on, and reads the audit's findings.
"""

import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import interchange_reader_audit as audit  # noqa: E402

ROOT = "C:/dinah-scratch/starved"


def call(name, **arguments):
    """One transcript line carrying one tool call, nested two levels down."""
    return {
        "type": "assistant",
        "message": {
            "content": [
                {"type": "text", "text": "working"},
                {"type": "tool_use", "id": "t1", "name": name, "input": arguments},
            ]
        },
    }


class AuditTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)

    def run_audit(self, *lines):
        transcript = Path(self.directory.name) / "round.jsonl"
        transcript.write_text(
            "".join(json.dumps(line) + "\n" for line in lines), encoding="utf-8"
        )
        out = Path(self.directory.name) / "audit.json"
        status = audit.main(["--root", ROOT, "--out", str(out), str(transcript)])
        return status, json.loads(out.read_text(encoding="utf-8"))

    def whys(self, report):
        return [(f["tool"], f["value"], f["why"]) for f in report["findings"]]

    def test_inside_paths_are_clean(self):
        status, report = self.run_audit(
            call("Read", file_path="C:\\Dinah-Scratch\\starved\\core-profile.md"),
            call("Write", file_path="out/reader.py", content="print(1)\n"),
            call("Glob", pattern="out/**/*.py"),
            call("Grep", pattern="CORE-", path="c:/dinah-scratch/starved"),
        )
        self.assertEqual(status, 0)
        self.assertEqual(report["tool_calls"], 4)
        self.assertEqual(
            report["by_tool"], {"Read": 1, "Write": 1, "Glob": 1, "Grep": 1}
        )
        self.assertEqual(report["findings"], [])

    def test_a_tool_outside_the_five_is_flagged(self):
        status, report = self.run_audit(call("Bash", command="ls"))
        self.assertEqual(status, 1)
        self.assertEqual(self.whys(report), [("Bash", "Bash", "tool-not-permitted")])

    def test_a_glob_path_beginning_with_a_tilde_is_flagged(self):
        status, report = self.run_audit(call("Glob", pattern="*", path="~/x"))
        self.assertEqual(status, 1)
        self.assertEqual(self.whys(report), [("Glob", "~/x", "home-relative")])

    def test_an_absolute_path_outside_the_root_is_flagged(self):
        status, report = self.run_audit(
            call("Read", file_path="C:/dinah-scratch/starved-old/x.md"),
            call("Glob", pattern="/etc/*"),
            call("Grep", pattern="x", path="\\\\server\\share"),
        )
        self.assertEqual(status, 1)
        self.assertEqual(
            self.whys(report),
            [
                ("Read", "C:/dinah-scratch/starved-old/x.md", "outside-root"),
                ("Glob", "/etc/*", "outside-root"),
                ("Grep", "\\\\server\\share", "outside-root"),
            ],
        )

    def test_a_parent_segment_is_flagged(self):
        status, report = self.run_audit(call("Grep", pattern="x", glob="out/../../*"))
        self.assertEqual(status, 1)
        self.assertEqual(
            self.whys(report),
            [("Grep", "out/../../*", "parent-segment")],
        )

    def test_file_text_and_search_text_are_not_read_as_paths(self):
        status, report = self.run_audit(
            call("Write", file_path="out/a.py", content="/usr/bin/env python"),
            call(
                "Edit",
                file_path="out/a.py",
                old_string="x",
                new_string="C:/Windows/system32",
            ),
            call("Grep", pattern="/[a-z]+/", path="out"),
        )
        self.assertEqual(status, 0)
        self.assertEqual(report["tool_calls"], 3)
        self.assertEqual(report["findings"], [])

    def test_a_stream_with_no_tool_call_exits_three(self):
        status, report = self.run_audit({"type": "result", "result": "done"})
        self.assertEqual(status, 3)
        self.assertEqual(report["tool_calls"], 0)


if __name__ == "__main__":
    unittest.main()
