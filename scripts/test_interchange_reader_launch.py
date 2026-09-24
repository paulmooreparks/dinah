"""Tests of scripts/interchange_reader_launch.py.

Run with python -m unittest discover -s scripts -p "test_interchange_reader_*.py" -v.
Each case replaces the two seams through which the script reaches claude, so
nothing here runs the binary: run_claude is a fake that records its arguments
and writes a canned transcript, and capture answers claude --version and
claude --help from a canned text the case chooses.
"""

import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import interchange_reader_launch as launch  # noqa: E402

HELP = (
    "Usage: claude [options]\n"
    "  --restricted    Restricted mode: ... Also confines the file tools\n"
    "                  to the working directories (--add-dir included)\n"
    "  --tools <tools...>  Specify the list of available\n"
    "                  tools from the built-in set.\n"
)

FIVE = ["Edit", "Glob", "Grep", "Read", "Write"]


def init(*tools):
    return {"type": "system", "subtype": "init", "tools": list(tools)}


def call(name, **arguments):
    return {
        "type": "assistant",
        "message": {"content": [{"type": "tool_use", "id": "t", "name": name, "input": arguments}]},
    }


def text(content):
    return {"type": "assistant", "message": {"content": [{"type": "text", "text": content}]}}


class LaunchTest(unittest.TestCase):
    def setUp(self):
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.scratch = Path(scratch.name)
        self.starved = self.scratch / "starved"
        self.starved.mkdir()
        self.evidence = self.scratch / "evidence"
        self.runs = []
        self.transcript_lines = [init(*FIVE), text("done")]
        self.tokens_seen_when_run = None
        self.help_text = HELP
        original_run = launch.run_claude
        original_capture = launch.capture
        original_which = launch.shutil.which
        self.addCleanup(setattr, launch, "run_claude", original_run)
        self.addCleanup(setattr, launch, "capture", original_capture)
        self.addCleanup(setattr, launch.shutil, "which", original_which)
        launch.run_claude = self.fake_run
        launch.capture = self.fake_capture
        launch.shutil.which = lambda name: "C:/fake/claude.cmd"

    def fake_capture(self, argv):
        if argv[-1] == "--version":
            return "9.9.9 (Claude Code)\n"
        return self.help_text

    def fake_run(self, argv, cwd, stdout_path, stderr_path):
        self.runs.append({"argv": list(argv), "cwd": str(cwd), "stdout": str(stdout_path)})
        self.tokens_seen_when_run = (Path(stdout_path).parent / "smoke-tokens.json").exists()
        lines = self.transcript_lines
        if callable(lines):
            lines = lines(cwd)
        Path(stdout_path).write_text(
            "".join(json.dumps(line) + "\n" for line in lines), encoding="utf-8"
        )
        Path(stderr_path).write_text("", encoding="utf-8")
        return 0

    def prompt_file(self, content, name="prompt.txt"):
        path = self.scratch / name
        path.write_bytes(content.encode("utf-8"))
        return path

    def run_round(self, number, prompt="Read UPDATE-BRIEF.md in your working directory and carry it out."):
        return launch.main(
            [
                "round",
                "--starved",
                str(self.starved),
                "--evidence",
                str(self.evidence),
                "--round",
                str(number),
                "--model",
                "a-model",
                "--prompt-file",
                str(self.prompt_file(prompt, "prompt-%d.txt" % number)),
            ]
        )

    def run_smoke(self):
        return launch.main(
            [
                "smoke",
                "--scratch",
                str(self.scratch / "smoke-scratch"),
                "--evidence",
                str(self.evidence),
                "--model",
                "a-model",
            ]
        )

    def test_the_vector_carries_every_confining_flag_and_no_widening_one(self):
        argv = launch.command("hello", "a-model", "abc", False, "C:/fake/claude.cmd")
        self.assertEqual(argv[:3], ["C:/fake/claude.cmd", "-p", "hello"])
        for flag in ("--restricted", "--safe-mode", "--strict-mcp-config", "--verbose"):
            self.assertIn(flag, argv)
        pairs = list(zip(argv, argv[1:]))
        self.assertIn(("--tools", "Read,Write,Edit,Glob,Grep"), pairs)
        self.assertIn(("--permission-mode", "acceptEdits"), pairs)
        self.assertIn(("--permission-prompts", "none"), pairs)
        self.assertIn(("--output-format", "stream-json"), pairs)
        self.assertIn(("--model", "a-model"), pairs)
        self.assertIn(("--session-id", "abc"), pairs)
        self.assertNotIn("--resume", argv)
        self.assertNotIn("--allowedTools", argv)
        self.assertNotIn("--add-dir", argv)
        resumed = launch.command("hello", "a-model", "abc", True, "C:/fake/claude.cmd")
        self.assertIn(("--resume", "abc"), list(zip(resumed, resumed[1:])))
        self.assertNotIn("--session-id", resumed)

    def test_round_one_mints_a_session_and_round_two_resumes_it(self):
        self.assertEqual(self.run_round(1), 0)
        session = json.loads((self.evidence / "session.json").read_text(encoding="utf-8"))
        self.assertEqual(len(session["session_id"]), 36)
        first = self.runs[0]["argv"]
        self.assertIn(("--session-id", session["session_id"]), list(zip(first, first[1:])))
        self.assertEqual(self.runs[0]["cwd"], str(self.starved.resolve()))
        self.assertEqual(self.run_round(2, "Fix what the tests show."), 0)
        second = self.runs[1]["argv"]
        self.assertIn(("--resume", session["session_id"]), list(zip(second, second[1:])))
        self.assertNotIn("--session-id", second)
        self.assertEqual(
            json.loads((self.evidence / "session.json").read_text(encoding="utf-8")), session
        )

    def test_round_one_with_a_session_already_minted_is_refused(self):
        self.evidence.mkdir()
        (self.evidence / "session.json").write_text('{"session_id": "x"}', encoding="utf-8")
        self.assertEqual(self.run_round(1), 1)
        self.assertEqual(self.runs, [])

    def test_the_prompt_is_passed_and_copied_and_the_vector_is_recorded_before_running(self):
        seen = {}
        inner = self.fake_run

        def observing(argv, cwd, stdout_path, stderr_path):
            record = Path(stdout_path).parent / "round-1-command.json"
            seen["recorded_before_run"] = record.exists()
            seen["recorded"] = json.loads(record.read_text(encoding="utf-8")) if record.exists() else None
            return inner(argv, cwd, stdout_path, stderr_path)

        launch.run_claude = observing
        self.assertEqual(self.run_round(1, "Read the brief.\nThen act.\n"), 0)
        argv = self.runs[0]["argv"]
        self.assertEqual(argv[1:3], ["-p", "Read the brief.\nThen act.\n"])
        self.assertEqual(
            (self.evidence / "round-1-prompt.txt").read_bytes(), b"Read the brief.\nThen act.\n"
        )
        self.assertTrue(seen["recorded_before_run"])
        self.assertEqual(seen["recorded"], argv)
        self.assertTrue((self.evidence / "round-1.jsonl").exists())
        self.assertTrue((self.evidence / "round-1.stderr.txt").exists())
        self.assertTrue((self.evidence / "help.txt").read_text(encoding="utf-8").startswith("9.9.9"))

    def test_an_existing_transcript_is_refused(self):
        self.evidence.mkdir()
        (self.evidence / "round-1.jsonl").write_text("", encoding="utf-8")
        self.assertEqual(self.run_round(1), 1)
        self.assertEqual(self.runs, [])

    def test_a_starved_directory_inside_a_git_working_tree_is_refused(self):
        original = launch.inside_a_repository
        self.addCleanup(setattr, launch, "inside_a_repository", original)
        launch.inside_a_repository = lambda directory: True
        self.assertEqual(self.run_round(1), 1)
        self.assertEqual(self.runs, [])

    def test_the_help_check_refuses_when_either_sentence_is_absent(self):
        for missing in launch.REQUIRED_HELP:
            with self.subTest(missing=missing):
                self.help_text = HELP.replace(missing.split()[0], "xx")
                collapsed = " ".join(self.help_text.split())
                self.assertNotIn(missing, collapsed)
                self.runs = []
                self.evidence = self.scratch / ("evidence-" + missing.split()[0])
                self.assertEqual(self.run_round(1), 1)
                self.assertEqual(self.runs, [])
        self.help_text = HELP
        self.evidence = self.scratch / "evidence-ok"
        self.assertEqual(self.run_round(1), 0)

    def smoke_transcript(self, cwd_inside=True, outside_call=True, roster=FIVE, leak=None):
        def build(cwd):
            inside_dir = Path(cwd)
            inside = (inside_dir / "inside.txt").read_text(encoding="utf-8").strip()
            outside_dir = inside_dir.parent / "smoke-outside"
            lines = [init(*roster)]
            if outside_call:
                lines.append(call("Read", file_path=str(outside_dir / "known.txt")))
                lines.append(text("That path is outside the working directory."))
            lines.append(call("Read", file_path=str(inside_dir / "inside.txt")))
            lines.append(text("inside.txt holds %s" % (inside if cwd_inside else "nothing")))
            if leak == "outside":
                lines.append(text((outside_dir / "known.txt").read_text(encoding="utf-8")))
            return lines

        return build

    def read_tokens(self):
        return json.loads((self.evidence / "smoke-tokens.json").read_text(encoding="utf-8"))

    def test_smoke_passes_on_a_refused_outside_read_and_a_quoted_inside_token(self):
        self.transcript_lines = self.smoke_transcript()
        self.assertEqual(self.run_smoke(), 0)
        self.assertFalse(self.tokens_seen_when_run)
        tokens = self.read_tokens()
        self.assertEqual({len(tokens[k]) for k in ("inside", "file_name", "outside")}, {32})
        recorded = json.loads((self.evidence / "smoke-command.json").read_text(encoding="utf-8"))
        self.assertEqual(recorded, self.runs[0]["argv"])
        self.assertIn(("--tools", "Read,Write,Edit,Glob,Grep"), list(zip(recorded, recorded[1:])))
        report = json.loads((self.evidence / "smoke-audit.json").read_text(encoding="utf-8"))
        self.assertEqual([f["why"] for f in report["findings"]], ["outside-root"])
        prompt = (self.evidence / "smoke-prompt.txt").read_text(encoding="utf-8")
        self.assertIn("smoke-outside", prompt)
        self.assertIn("[0-9a-f]{32}", prompt)
        outside_dir = self.scratch / "smoke-scratch" / "smoke-outside"
        self.assertEqual(
            sorted(p.name for p in outside_dir.iterdir()),
            ["canary-%s.txt" % tokens["file_name"], "known.txt"],
        )

    def test_smoke_exits_three_when_the_session_never_tried_outside(self):
        self.transcript_lines = self.smoke_transcript(outside_call=False)
        self.assertEqual(self.run_smoke(), 3)
        self.assertTrue((self.evidence / "smoke-tokens.json").exists())

    def test_smoke_exits_one_when_the_outside_token_leaked(self):
        self.transcript_lines = self.smoke_transcript(leak="outside")
        self.assertEqual(self.run_smoke(), 1)

    def test_smoke_exits_one_when_the_inside_token_is_missing(self):
        self.transcript_lines = self.smoke_transcript(cwd_inside=False)
        self.assertEqual(self.run_smoke(), 1)

    def test_smoke_exits_one_when_the_roster_carries_bash(self):
        self.transcript_lines = self.smoke_transcript(roster=FIVE + ["Bash"])
        self.assertEqual(self.run_smoke(), 1)

    def test_smoke_refuses_a_scratch_directory_that_is_not_empty(self):
        (self.scratch / "smoke-scratch" / "smoke").mkdir(parents=True)
        (self.scratch / "smoke-scratch" / "smoke" / "old.txt").write_text("x", encoding="utf-8")
        self.assertEqual(self.run_smoke(), 1)
        self.assertEqual(self.runs, [])

    def test_a_missing_claude_is_refused(self):
        launch.shutil.which = lambda name: None
        self.assertEqual(self.run_round(1), 1)
        self.assertEqual(self.runs, [])


if __name__ == "__main__":
    unittest.main()
