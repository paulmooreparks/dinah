"""Unit tests of the interchange reader.

Run from the directory holding out/ with

    python -m unittest discover -s out/tests -t out -v

and INTERCHANGE_READER_PROFILE naming the profile. Every test drives
reader.py as a separate process, the way the brief's command line does.
"""

import copy
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

OUT = Path(__file__).resolve().parent.parent
READER = OUT / "reader.py"
SCOPE = OUT / "scope.json"
FIXTURES = OUT / "fixtures"
EXPECTATIONS = FIXTURES / "expectations.json"
PROFILE_VARIABLE = "INTERCHANGE_READER_PROFILE"

# Section 3.2 of the profile.
STATEMENT_RE = re.compile(r"^\[([A-Z][A-Z0-9]*(-[A-Z0-9]+)*)\] (.+)$")
FIXTURE_NAME_RE = re.compile(r"^(accept|refuse)-[a-z0-9-]+\.json$")

# The example of section 5.7, with one unrecognized member at each level and
# one layer, used as the sent object of the pair tests.
SENT = {
    "profile": "dinah-core/0.17",
    "title": "Wedding",
    "columns": [
        {"id": "s1", "title": "Ideas", "kind": "intake", "colour": "green"},
        {"id": "s2", "title": "Deciding", "kind": "work",
         "instructions": "Pick one and say why.", "capacity": 3},
        {"id": "s3", "title": "Booked", "kind": "done"},
    ],
    "notes": "Keep every receipt.",
    "acme.flow": {"buffer": "s2"},
}


def failing(result):
    return {s["id"] for s in result["statements"] if s["result"] == "fail"}


def results_by_id(result):
    return {s["id"]: s["result"] for s in result["statements"]}


class ReaderTest(unittest.TestCase):
    """Base class: every test fails when the profile variable is unset."""

    def setUp(self):
        profile = os.environ.get(PROFILE_VARIABLE)
        if not profile:
            self.fail(f"{PROFILE_VARIABLE} is not set; the tests need the path of the profile")
        self.profile = os.path.abspath(profile)
        self.tmp = Path(tempfile.mkdtemp(prefix="reader-test-"))
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)

    # Running the reader

    def run_reader(self, *args, reader=READER, profile=True):
        command = [sys.executable, str(reader)]
        if profile is True:
            command += ["--profile", self.profile]
        elif profile:
            command += ["--profile", str(profile)]
        command += [str(arg) for arg in args]
        return subprocess.run(command, capture_output=True, timeout=120)

    def output(self, *args, **kwargs):
        proc = self.run_reader(*args, **kwargs)
        self.assertEqual(proc.returncode, 0, proc.stderr.decode("utf-8", "replace"))
        return json.loads(proc.stdout.decode("utf-8"))

    def check_one(self, path):
        document = self.output("check", path)
        self.assertEqual(len(document["results"]), 1)
        return document["results"][0]

    def write(self, name, content):
        path = self.tmp / name
        if not isinstance(content, bytes):
            content = json.dumps(content, indent=2).encode("utf-8")
        path.write_bytes(content)
        return path

    # What the profile and scope.json say

    def profile_text(self):
        return Path(self.profile).read_bytes().decode("utf-8")

    def published(self):
        identifiers = []
        for line in self.profile_text().split("\n"):
            match = STATEMENT_RE.match(line.rstrip("\r"))
            if match and match.group(1) not in identifiers:
                identifiers.append(match.group(1))
        return identifiers

    def version_identity(self):
        match = re.search(r"Version identity: `([^`]+)`", self.profile_text())
        self.assertIsNotNone(match, "the profile states no version identity")
        return match.group(1)

    def scope_entries(self):
        return json.loads(SCOPE.read_text(encoding="utf-8"))["statements"]

    def scoped(self, scope):
        entries = self.scope_entries()
        return [i for i in self.published() if entries.get(i, {}).get("scope") == scope]

    def reader_copy(self, edit):
        """A copy of the reader beside a scope.json that edit has changed."""
        target = self.tmp / "copy"
        target.mkdir()
        for module in OUT.glob("*.py"):
            shutil.copy(module, target / module.name)
        scope = json.loads(SCOPE.read_text(encoding="utf-8"))
        edit(scope["statements"])
        (target / "scope.json").write_text(json.dumps(scope, indent=2), encoding="utf-8")
        return target / "reader.py"


class ProfileVariableTest(ReaderTest):
    def test_variable_names_a_readable_profile(self):
        self.assertTrue(Path(self.profile).is_file(), f"{self.profile} is not a file")
        self.assertTrue(self.published(), "the profile yields no statements")


class FixtureTest(ReaderTest):
    def expectations(self):
        return json.loads(EXPECTATIONS.read_text(encoding="utf-8"))["fixtures"]

    def test_every_fixture_file_is_listed_exactly_once(self):
        listed = [entry["file"] for entry in self.expectations()]
        on_disk = sorted(p.name for p in FIXTURES.iterdir() if p.name != "expectations.json")
        self.assertEqual(sorted(listed), on_disk)
        self.assertEqual(len(listed), len(set(listed)), "a fixture is listed twice")

    def test_fixture_names_follow_the_brief(self):
        for entry in self.expectations():
            self.assertRegex(entry["file"], FIXTURE_NAME_RE)
            self.assertTrue(entry["file"].startswith(entry["expect"] + "-"), entry["file"])

    def test_listed_statements_are_classified_export(self):
        export = set(self.scoped("export"))
        for entry in self.expectations():
            with self.subTest(fixture=entry["file"]):
                self.assertTrue(entry["statements"], "a fixture names no statement")
                self.assertLessEqual(set(entry["statements"]), export)

    def test_every_fixture_meets_its_expectation(self):
        entries = self.expectations()
        document = self.output("check", *[FIXTURES / e["file"] for e in entries])
        self.assertEqual(len(document["results"]), len(entries))
        for entry, result in zip(entries, document["results"]):
            with self.subTest(fixture=entry["file"]):
                self.assertEqual(result["file"], str(FIXTURES / entry["file"]))
                self.assertEqual(result["verdict"], entry["expect"])
                self.assertEqual(result["refusal"], entry["refusal"])
                if entry["expect"] == "refuse":
                    # A refuse fixture breaks the statements it lists and nothing else.
                    self.assertEqual(failing(result), set(entry["statements"]))
                else:
                    self.assertEqual(failing(result), set())
                    results = results_by_id(result)
                    for ident in entry["statements"]:
                        self.assertEqual(results[ident], "pass", ident)

    def test_every_export_statement_has_a_satisfying_fixture(self):
        satisfied = set()
        for entry in self.expectations():
            if entry["expect"] == "accept":
                satisfied |= set(entry["statements"])
        for ident in self.scoped("export"):
            with self.subTest(statement=ident):
                self.assertIn(ident, satisfied)

    def test_every_export_must_has_a_breaking_fixture(self):
        texts = {}
        for line in self.profile_text().split("\n"):
            match = STATEMENT_RE.match(line.rstrip("\r"))
            if match:
                texts.setdefault(match.group(1), match.group(3))
        broken = set()
        for entry in self.expectations():
            if entry["expect"] == "refuse" and len(entry["statements"]) == 1:
                broken |= set(entry["statements"])
        for ident in self.scoped("export"):
            if re.search(r"\bMUST\b", texts[ident]):
                with self.subTest(statement=ident):
                    self.assertIn(ident, broken)


class OutputShapeTest(ReaderTest):
    def test_reader_member_names_the_profile(self):
        document = self.output("check", FIXTURES / "accept-section-5-7-example.json")
        expected_hash = hashlib.sha256(Path(self.profile).read_bytes()).hexdigest()
        self.assertEqual(document["reader"], {
            "name": "interchange-reader",
            "profile": self.version_identity(),
            "profile_sha256": expected_hash,
        })

    def test_statements_are_the_export_scope_in_published_order(self):
        result = self.check_one(FIXTURES / "accept-section-5-7-example.json")
        self.assertEqual([s["id"] for s in result["statements"]], self.scoped("export"))
        for statement in result["statements"]:
            self.assertIn(statement["result"], ("pass", "fail", "not-applicable"))
            self.assertIsInstance(statement["detail"], str)
            self.assertTrue(statement["detail"].endswith("."), statement["detail"])

    def test_results_keep_the_order_files_were_given(self):
        names = ["refuse-missing-title.json", "accept-section-5-7-example.json",
                 "refuse-columns-empty.json"]
        document = self.output("check", *[FIXTURES / n for n in names])
        self.assertEqual([r["file"] for r in document["results"]],
                         [str(FIXTURES / n) for n in names])

    def test_stdout_is_one_utf8_json_document(self):
        proc = self.run_reader("check", FIXTURES / "accept-wedding-walkthrough.json")
        self.assertEqual(proc.returncode, 0)
        document = json.loads(proc.stdout.decode("utf-8"))
        self.assertEqual(set(document), {"reader", "results"})


class ByteLevelTest(ReaderTest):
    """Cases built from bytes, which a fixture file cannot hold as cleanly."""

    def example_bytes(self):
        return (FIXTURES / "accept-section-5-7-example.json").read_bytes()

    def assert_refused_by_json_2_alone(self, result):
        self.assertEqual(result["verdict"], "refuse")
        self.assertEqual(result["refusal"], "malformed")
        self.assertEqual(failing(result), {"CORE-JSON-2"})
        for statement in result["statements"]:
            if statement["id"] != "CORE-JSON-2":
                self.assertEqual(statement["result"], "not-applicable", statement["id"])

    def test_bytes_that_are_not_utf8(self):
        path = self.write("latin1.json", b'{"profile": "dinah-core/0.17", "title": "Caf\xe9", "columns": []}')
        self.assert_refused_by_json_2_alone(self.check_one(path))

    def test_utf16_with_its_byte_order_mark(self):
        path = self.write("utf16.json", self.example_bytes().decode("utf-8").encode("utf-16"))
        self.assert_refused_by_json_2_alone(self.check_one(path))

    def test_utf8_byte_order_mark_is_ignored(self):
        path = self.write("bom.json", b"\xef\xbb\xbf" + self.example_bytes())
        result = self.check_one(path)
        self.assertEqual(result["verdict"], "accept")
        self.assertEqual(results_by_id(result)["CORE-JSON-2"], "pass")

    def test_two_byte_order_marks_are_not_json(self):
        path = self.write("bom2.json", b"\xef\xbb\xbf\xef\xbb\xbf" + self.example_bytes())
        self.assert_refused_by_json_2_alone(self.check_one(path))

    def test_text_that_is_not_json(self):
        path = self.write("prose.json", b"profile: dinah-core/0.17\ntitle: Wedding\n")
        self.assert_refused_by_json_2_alone(self.check_one(path))

    def test_empty_file(self):
        self.assert_refused_by_json_2_alone(self.check_one(self.write("empty.json", b"")))

    def test_truncated_object(self):
        path = self.write("truncated.json", self.example_bytes()[:40])
        self.assert_refused_by_json_2_alone(self.check_one(path))

    def test_trailing_text_after_the_object(self):
        path = self.write("trailing.json", self.example_bytes() + b"{}")
        self.assert_refused_by_json_2_alone(self.check_one(path))

    def test_nan_is_not_a_json_value(self):
        text = self.example_bytes().replace(b'"capacity": 3', b'"capacity": NaN')
        self.assert_refused_by_json_2_alone(self.check_one(self.write("nan.json", text)))

    def test_crlf_line_endings_are_json_whitespace(self):
        path = self.write("crlf.json", self.example_bytes().replace(b"\n", b"\r\n"))
        self.assertEqual(self.check_one(path)["verdict"], "accept")

    def test_repeated_member_name_keeps_the_last_value(self):
        text = b'{"profile": "dinah-core/0.17", "title": "", "title": "Wedding", ' \
               b'"columns": [{"id": "s1", "title": "Ideas", "kind": "intake"}]}'
        result = self.check_one(self.write("repeat.json", text))
        self.assertEqual(result["verdict"], "accept")

    def test_true_is_not_a_capacity(self):
        text = self.example_bytes().replace(b'"capacity": 3', b'"capacity": true')
        result = self.check_one(self.write("capacity-true.json", text))
        self.assertEqual(failing(result), {"CORE-STATE-5"})

    def test_unreadable_files_still_yield_results_beside_readable_ones(self):
        bad = self.write("bad.json", b"\xff\xfe\xfd")
        document = self.output("check", bad, FIXTURES / "accept-section-5-7-example.json")
        self.assertEqual([r["verdict"] for r in document["results"]], ["refuse", "accept"])


class PairTest(ReaderTest):
    def run_pair(self, returned, sent=SENT):
        sent_path = self.write("sent.json", sent)
        returned_path = self.write("returned.json", returned)
        document = self.output("pair", sent_path, returned_path)
        self.assertEqual(set(document), {"reader", "result"})
        result = document["result"]
        self.assertEqual(result["sent"], str(sent_path))
        self.assertEqual(result["returned"], str(returned_path))
        self.assertEqual([s["id"] for s in result["statements"]], self.scoped("pair"))
        return result

    def test_pair_scope_is_what_the_reader_judges(self):
        self.assertEqual(self.scoped("pair"),
                         ["CORE-TEXT-1", "CORE-TEXT-3", "CORE-JSON-1", "CORE-JSON-7", "CORE-LAYER-2"])

    def test_faithful_rewrite_passes_everything(self):
        rewritten = dict(reversed(list(copy.deepcopy(SENT).items())))
        text = json.dumps(rewritten, separators=(",", ":")).replace('"capacity":3', '"capacity":3.0')
        result = self.run_pair(text.encode("utf-8"))
        self.assertEqual(set(results_by_id(result).values()), {"pass"})

    def test_added_slugs_are_not_a_change(self):
        returned = copy.deepcopy(SENT)
        for column in returned["columns"]:
            column["slug"] = column["title"].lower()
        self.assertEqual(set(results_by_id(self.run_pair(returned)).values()), {"pass"})

    def test_dropped_top_level_member_fails_json_7(self):
        returned = copy.deepcopy(SENT)
        del returned["notes"]
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-JSON-7"})

    def test_dropped_column_member_fails_json_7(self):
        returned = copy.deepcopy(SENT)
        del returned["columns"][0]["colour"]
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-JSON-7"})

    def test_dropped_layer_fails_layer_2(self):
        returned = copy.deepcopy(SENT)
        del returned["acme.flow"]
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-LAYER-2"})

    def test_changed_layer_content_fails_layer_2(self):
        returned = copy.deepcopy(SENT)
        returned["acme.flow"] = {"buffer": "s3"}
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-LAYER-2"})

    def test_reordered_columns_fail_json_1(self):
        returned = copy.deepcopy(SENT)
        returned["columns"].reverse()
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-JSON-1"})

    def test_dropped_capacity_fails_json_1(self):
        returned = copy.deepcopy(SENT)
        del returned["columns"][1]["capacity"]
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-JSON-1"})

    def test_translated_column_member_name_fails_text_3(self):
        returned = copy.deepcopy(SENT)
        returned["columns"][1]["capacite"] = returned["columns"][1].pop("capacity")
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-JSON-1", "CORE-TEXT-3"})

    def test_translated_kind_fails_text_3(self):
        returned = copy.deepcopy(SENT)
        returned["columns"][1]["kind"] = "travail"
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-JSON-1", "CORE-TEXT-3"})

    def test_translated_member_name_fails_text_3(self):
        returned = copy.deepcopy(SENT)
        returned["titre"] = returned.pop("title")
        self.assertEqual(failing(self.run_pair(returned)), {"CORE-JSON-1", "CORE-TEXT-3"})

    def test_returned_bytes_not_utf8(self):
        returned = json.dumps(SENT).replace("Wedding", "Mariage \udcff").encode("utf-8", "surrogateescape")
        result = self.run_pair(returned)
        self.assertEqual(failing(result), {"CORE-TEXT-1", "CORE-JSON-1", "CORE-JSON-7", "CORE-LAYER-2"})
        self.assertEqual(results_by_id(result)["CORE-TEXT-3"], "not-applicable")

    def test_unreadable_sent_file_leaves_comparisons_not_applicable(self):
        result = self.run_pair(SENT, sent=b"not json")
        results = results_by_id(result)
        self.assertEqual(results.pop("CORE-TEXT-1"), "pass")
        self.assertEqual(set(results.values()), {"not-applicable"})

    def test_nothing_to_preserve_is_not_applicable(self):
        sent = json.loads((FIXTURES / "accept-section-5-7-example.json").read_text(encoding="utf-8"))
        result = self.run_pair(sent, sent=sent)
        results = results_by_id(result)
        self.assertEqual(results["CORE-JSON-7"], "not-applicable")
        self.assertEqual(results["CORE-LAYER-2"], "not-applicable")
        self.assertEqual(results["CORE-JSON-1"], "pass")


class ScopeTest(ReaderTest):
    def test_scope_json_classifies_every_core_statement(self):
        entries = self.scope_entries()
        for ident in self.published():
            if ident.startswith("CORE-"):
                with self.subTest(statement=ident):
                    self.assertIn(ident, entries)
                    self.assertIn(entries[ident]["scope"], ("export", "pair", "none"))
                    self.assertTrue(entries[ident]["reason"].endswith("."))

    def test_scope_json_names_the_profile_version(self):
        scope = json.loads(SCOPE.read_text(encoding="utf-8"))
        self.assertEqual(scope["profile"], self.version_identity())

    def test_scope_command_reports_nothing_missing(self):
        proc = self.run_reader("scope")
        self.assertEqual(proc.returncode, 0, proc.stderr.decode("utf-8", "replace"))
        document = json.loads(proc.stdout.decode("utf-8"))
        entries = self.scope_entries()
        counts = {s: sum(1 for e in entries.values() if e["scope"] == s) for s in ("export", "pair", "none")}
        self.assertEqual(document["classified"], counts)
        self.assertEqual(document["unclassified"], [])
        self.assertEqual(document["stale"], [])
        self.assertEqual(document["reader"]["profile"], self.version_identity())

    def test_scope_command_reports_stale_identifiers(self):
        def add(statements):
            statements["CORE-GONE-1"] = {"scope": "none", "reason": "Retired."}
        document = self.output("scope", reader=self.reader_copy(add))
        self.assertEqual(document["stale"], ["CORE-GONE-1"])

    def test_unclassified_statement_outside_json_family_still_exits_0(self):
        def remove(statements):
            del statements["CORE-QUEUE-3"]
        document = self.output("scope", reader=self.reader_copy(remove))
        self.assertEqual(document["unclassified"], ["CORE-QUEUE-3"])


class ExitCodeTest(ReaderTest):
    def assert_exit(self, proc, code):
        self.assertEqual(proc.returncode, code, proc.stderr.decode("utf-8", "replace"))

    def test_0_for_a_refused_object(self):
        proc = self.run_reader("check", FIXTURES / "refuse-missing-title.json")
        self.assert_exit(proc, 0)
        self.assertEqual(json.loads(proc.stdout)["results"][0]["verdict"], "refuse")

    def test_0_for_pair_and_scope(self):
        self.assert_exit(self.run_reader("pair", FIXTURES / "accept-section-5-7-example.json",
                                         FIXTURES / "accept-section-5-7-example.json"), 0)
        self.assert_exit(self.run_reader("scope"), 0)

    def test_2_for_command_lines_not_understood(self):
        fixture = FIXTURES / "accept-section-5-7-example.json"
        cases = {
            "no arguments": ([], False),
            "no command": ([], True),
            "no --profile": (["check", fixture], False),
            "unknown command": (["validate", fixture], True),
            "check without a file": (["check"], True),
            "pair with one file": (["pair", fixture], True),
            "pair with three files": (["pair", fixture, fixture, fixture], True),
            "scope with a file": (["scope", fixture], True),
            "a file that does not exist": (["check", self.tmp / "absent.json"], True),
        }
        for label, (args, profile) in cases.items():
            with self.subTest(case=label):
                proc = self.run_reader(*args, profile=profile)
                self.assert_exit(proc, 2)
                self.assertEqual(proc.stdout, b"")

    def test_3_when_the_profile_cannot_be_used(self):
        empty = self.write("empty-profile.md",
                           b"# A profile\n\nVersion identity: `dinah-core 0.17`, maturity channel `dev`.\n")
        latin1 = self.write("latin1-profile.md", b"# Caf\xe9\n\n[CORE-X-1] A tool MUST exist.\n")
        cases = {
            "missing": self.tmp / "absent.md",
            "no statements": empty,
            "not UTF-8": latin1,
        }
        for label, path in cases.items():
            for command in (["scope"], ["check", FIXTURES / "accept-section-5-7-example.json"]):
                with self.subTest(case=label, command=command[0]):
                    proc = self.run_reader(*command, profile=path)
                    self.assert_exit(proc, 3)
                    self.assertEqual(proc.stdout, b"")

    def test_4_when_a_json_statement_is_unclassified(self):
        def remove(statements):
            del statements["CORE-JSON-7"]
        proc = self.run_reader("scope", reader=self.reader_copy(remove))
        self.assert_exit(proc, 4)
        document = json.loads(proc.stdout.decode("utf-8"))
        self.assertEqual(document["unclassified"], ["CORE-JSON-7"])


if __name__ == "__main__":
    unittest.main()
