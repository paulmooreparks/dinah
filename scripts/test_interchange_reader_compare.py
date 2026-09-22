"""Tests of scripts/interchange_reader_compare.py.

Run with python -m unittest discover -s scripts -p "test_interchange_reader_*.py" -v.
Each case builds a repository root of its own in a temporary directory, with a
stub reader and a stub Dinah whose answers the case sets, so no case needs a Go
build or the real reader. The last case reads the real reader's directory and
checks that it imports nothing outside the standard library.
"""

import ast
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import interchange_reader_compare as compare  # noqa: E402

REPOSITORY = Path(__file__).resolve().parents[1]

PROFILE_TEXT = (
    "# A profile\n\nVersion identity: `x 0.1`\n\n"
    "Members: `profile`, `title`, `columns`, `id`, `kind`.\n"
)

# Every statement the stub reader fails on a mutant unless told otherwise.
MUTANT_STATEMENTS = sorted({statement for _, _, statement in compare.MUTANTS})

STUB_READER = r'''
import json, sys
from pathlib import Path
here = Path(__file__).resolve().parent
config = json.loads((here / "stub-config.json").read_text(encoding="utf-8"))
reader = {"name": "interchange-reader", "profile": "x 0.1", "profile_sha256": "0"}
args = sys.argv[1:]
assert args[0] == "--profile"
command, files = args[2], args[3:]
def result(path):
    p = Path(path)
    if p.name in config.get("check", {}):
        given = dict(config["check"][p.name])
    elif p.parent.name == "mutants":
        given = {"verdict": "refuse", "refusal": "malformed",
                 "statements": [{"id": s, "result": "fail", "detail": "broken."}
                                for s in config["mutant_statements"]]}
    else:
        given = {"verdict": "accept", "refusal": None,
                 "statements": [{"id": "S-1", "result": "pass", "detail": "fine."}]}
    given["file"] = path
    return given
if command == "check":
    print(json.dumps({"reader": reader, "results": [result(f) for f in files]}))
elif command == "pair":
    print(json.dumps({"reader": reader, "result": {"sent": files[0], "returned": files[1],
        "statements": config.get("pair", [{"id": "S-2", "result": "pass", "detail": "kept."}])}}))
elif command == "scope":
    print(json.dumps({"reader": reader, "classified": {"export": 1, "pair": 1, "none": 0},
        "unclassified": config.get("unclassified", []), "stale": []}))
    sys.exit(config.get("scope_exit", 0))
'''

STUB_DINAH = r'''
import json, shutil, sys
from pathlib import Path
config = json.loads((Path(__file__).resolve().parent / "dinah-config.json").read_text(encoding="utf-8"))
args = sys.argv[1:]
assert args[:2] == ["--format", "json"]
args = args[2:]
if args[0] == "init":
    target, source = Path(args[1]), Path(args[3])
    refuse = config.get("refuse", {})
    if source.name in refuse or (source.parent.name == "mutants" and source.name not in config.get("admit", [])):
        print(json.dumps({"outcome": "refused", "refusal": refuse.get(source.name, "malformed")}))
        sys.exit(2)
    workbench = target / ".dinah" / "abc"
    workbench.mkdir(parents=True)
    shutil.copyfile(source, workbench / "definition.json")
    sys.exit(0)
if args[0] == "--workbench" and args[2] == "export":
    sys.stdout.buffer.write((Path(args[1]) / "definition.json").read_bytes())
    sys.exit(0)
sys.exit(9)
'''


def sample_export():
    return {
        "profile": "x/0.1",
        "title": "Sample",
        "secret": 1,
        "columns": [
            {"id": "a", "title": "A", "kind": "intake"},
            {"id": "b", "title": "B", "kind": "work"},
        ],
    }


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2), encoding="utf-8")


class CompareTest(unittest.TestCase):
    def setUp(self):
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.base = Path(scratch.name)
        self.root = self.base / "root"
        self.reader_dir = self.root / "conformance" / "interchange-reader"
        (self.reader_dir / "fixtures").mkdir(parents=True)
        (self.reader_dir / "reader.py").write_text(STUB_READER, encoding="utf-8")
        self.reader_config = {"mutant_statements": MUTANT_STATEMENTS}
        write_json(
            self.reader_dir / "scope.json",
            {"profile": "x 0.1", "statements": {"S-1": {"scope": "export"}, "S-2": {"scope": "pair"}}},
        )
        fixture = sample_export()
        del fixture["secret"]
        write_json(self.reader_dir / "fixtures" / "accept-one.json", fixture)
        write_json(self.reader_dir / "fixtures" / "refuse-two.json", {"title": "no columns"})
        write_json(
            self.reader_dir / "fixtures" / "expectations.json",
            {
                "fixtures": [
                    {"file": "accept-one.json", "expect": "accept", "refusal": None, "statements": ["S-1"]},
                    {"file": "refuse-two.json", "expect": "refuse", "refusal": None, "statements": ["S-1"]},
                ]
            },
        )
        self.reader_config["check"] = {
            "refuse-two.json": {
                "verdict": "refuse",
                "refusal": "malformed",
                "statements": [{"id": "S-1", "result": "fail", "detail": "no columns."}],
            }
        }
        (self.root / "docs" / "spec").mkdir(parents=True)
        (self.root / "docs" / "spec" / "core-profile.md").write_text(PROFILE_TEXT, encoding="utf-8")
        write_json(
            self.root / "internal" / "bench" / "testdata" / "compat" / "manifest.json",
            {"fixtures": [{"directory": "old"}, {"directory": "new", "sample": True}]},
        )
        self.exports = self.base / "exports"
        write_json(self.exports / "old.json", sample_export())
        write_json(self.exports / "new.json", sample_export())
        self.conformance = self.base / "conformance.json"
        write_json(
            self.conformance,
            {
                "profile": "x 0.1",
                "statements": [
                    {"id": "S-1", "tests": ["TestA"], "out_of_reach": None},
                    {"id": "S-2", "tests": [], "out_of_reach": "a reason"},
                ],
            },
        )
        self.dinah = self.base / "stub-dinah" / "dinah.py"
        self.dinah.parent.mkdir()
        self.dinah.write_text(STUB_DINAH, encoding="utf-8")
        self.dinah_config = {"refuse": {"refuse-two.json": "malformed"}}
        # The default tree agrees everywhere: the stub Dinah refuses every
        # mutant and the stub reader fails each on every statement a mutant
        # breaks, and no file of the reader quotes the member the profile
        # never names.
        self.rulings = []

    def ruling(self, key, ruled_by="paul", disposition="both-conform"):
        return {"key": key, "disposition": disposition, "card": None, "item": "x", "ruled_by": ruled_by, "date": "2026-09-21", "note": "n"}

    def run_compare(self):
        (self.reader_dir / "stub-config.json").write_text(json.dumps(self.reader_config), encoding="utf-8")
        write_json(self.dinah.parent / "dinah-config.json", self.dinah_config)
        rulings = self.base / "rulings.json"
        write_json(rulings, {"rulings": self.rulings})
        out = self.base / "out"
        status = compare.main(
            [
                "--dinah", str(self.dinah),
                "--exports", str(self.exports),
                "--conformance", str(self.conformance),
                "--rulings", str(rulings),
                "--out", str(out),
                "--root", str(self.root),
            ]
        )
        report_path = out / "report.json"
        report = json.loads(report_path.read_text(encoding="utf-8")) if report_path.exists() else None
        return status, report

    def keys(self, report):
        return sorted(d["key"] for d in report["disagreements"])

    def test_a_tree_that_agrees_everywhere_passes(self):
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), [])
        self.assertEqual(status, 0)
        self.assertEqual(report["counts"]["manifest_fixtures"], 2)
        self.assertEqual(report["counts"]["exports"], 2)
        self.assertEqual(report["counts"]["mutants"], 13)
        self.assertEqual(report["counts"]["reader_fixtures"], 2)
        self.assertEqual(report["counts"]["conformance_statements"], 2)
        self.assertEqual(report["leak_names"], ["secret"])
        self.assertEqual([row["id"] for row in report["statements"]], ["S-1", "S-2"])
        self.assertEqual(report["statements"][0]["exports"], {"pass": 2, "fail": 0, "not_applicable": 0})

    def test_an_export_the_reader_fails_is_a_disagreement(self):
        self.reader_config["check"]["old.json"] = {
            "verdict": "refuse",
            "refusal": "malformed",
            "statements": [{"id": "S-1", "result": "fail", "detail": "wrong."}],
        }
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), ["export:old:S-1"])
        self.assertEqual(status, 1)

    def test_an_export_refused_with_no_failing_statement_is_keyed_verdict(self):
        self.reader_config["check"]["old.json"] = {"verdict": "refuse", "refusal": None, "statements": []}
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), ["export:old:verdict"])
        self.assertEqual(status, 1)

    def test_a_fixture_dinah_answers_differently_is_a_disagreement(self):
        self.dinah_config["refuse"]["accept-one.json"] = "malformed"
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), ["fixture:accept-one.json:verdict"])
        self.assertEqual(status, 1)

    def test_a_fixture_refused_under_another_name_is_a_disagreement(self):
        expectations = self.reader_dir / "fixtures" / "expectations.json"
        data = json.loads(expectations.read_text(encoding="utf-8"))
        data["fixtures"][1]["refusal"] = "malformed"
        write_json(expectations, data)
        self.dinah_config["refuse"]["refuse-two.json"] = "duplicate"
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), ["fixture:refuse-two.json:verdict"])

    def test_a_pair_statement_the_reader_fails_is_a_disagreement(self):
        self.reader_config["pair"] = [{"id": "S-2", "result": "fail", "detail": "lost."}]
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), ["pair:accept-one.json:S-2"])
        self.assertEqual(status, 1)

    def test_a_mutant_either_side_accepts_is_a_disagreement(self):
        self.dinah_config["admit"] = ["drop-title.json"]
        self.reader_config["check"]["empty-columns.json"] = {"verdict": "accept", "refusal": None, "statements": []}
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), ["mutant:drop-title:dinah", "mutant:empty-columns:reader"])
        self.assertEqual(status, 1)

    def test_a_reader_contradicting_its_own_expectation_fails_and_cannot_be_ruled(self):
        del self.reader_config["check"]["refuse-two.json"]
        status, report = self.run_compare()
        self.assertEqual([s["key"] for s in report["self_inconsistent"]], ["self:refuse-two.json"])
        self.assertEqual(status, 1)

    def test_a_quoted_member_name_the_profile_never_names_is_a_leak(self):
        (self.reader_dir / "extra.py").write_text("x = 1\nname = \"secret\"\n", encoding="utf-8")
        (self.reader_dir / "README.md").write_text("'secret' is fine here\n", encoding="utf-8")
        status, report = self.run_compare()
        self.assertEqual(self.keys(report), ["leak:extra.py:2:secret"])
        self.assertEqual(status, 1)

    def test_a_ruled_key_passes(self):
        self.reader_config["pair"] = [{"id": "S-2", "result": "fail", "detail": "lost."}]
        self.rulings.append(self.ruling("pair:accept-one.json:S-2"))
        status, report = self.run_compare()
        self.assertEqual(status, 0)
        self.assertEqual(report["counts"]["ruled"], 1)
        self.assertEqual(report["counts"]["unruled"], 0)

    def test_a_ruling_with_an_empty_ruled_by_fails(self):
        self.reader_config["pair"] = [{"id": "S-2", "result": "fail", "detail": "lost."}]
        self.rulings.append(self.ruling("pair:accept-one.json:S-2", ruled_by=""))
        status, report = self.run_compare()
        self.assertEqual(status, 1)
        self.assertEqual(report["unruled"], [{"key": "pair:accept-one.json:S-2", "why": "the ruling's ruled_by is empty"}])

    def test_a_not_a_leak_ruling_on_another_key_fails(self):
        self.reader_config["pair"] = [{"id": "S-2", "result": "fail", "detail": "lost."}]
        self.rulings.append(self.ruling("pair:accept-one.json:S-2", disposition="not-a-leak"))
        status, _ = self.run_compare()
        self.assertEqual(status, 1)

    def test_a_key_ruled_twice_fails(self):
        self.reader_config["pair"] = [{"id": "S-2", "result": "fail", "detail": "lost."}]
        self.rulings.append(self.ruling("pair:accept-one.json:S-2"))
        self.rulings.append(self.ruling("pair:accept-one.json:S-2", disposition="tool-defect"))
        status, report = self.run_compare()
        self.assertEqual(status, 1)
        self.assertEqual(
            report["bad_rulings"],
            ["pair:accept-one.json:S-2: the rulings file rules this key more than once"],
        )

    def test_a_defect_ruling_naming_a_card_passes(self):
        self.reader_config["pair"] = [{"id": "S-2", "result": "fail", "detail": "lost."}]
        ruling = self.ruling("pair:accept-one.json:S-2", disposition="tool-defect")
        ruling["card"] = "dinah-1"
        self.rulings.append(ruling)
        status, report = self.run_compare()
        self.assertEqual(status, 0)
        self.assertEqual(report["bad_rulings"], [])

    def test_a_defect_ruling_naming_no_card_fails(self):
        self.reader_config["pair"] = [{"id": "S-2", "result": "fail", "detail": "lost."}]
        for disposition in ("tool-defect", "profile-defect", "reader-defect"):
            with self.subTest(disposition=disposition):
                self.rulings = [self.ruling("pair:accept-one.json:S-2", disposition=disposition)]
                status, report = self.run_compare()
                self.assertEqual(status, 1)
                self.assertEqual(
                    report["bad_rulings"],
                    ["pair:accept-one.json:S-2: a %s ruling must name the card that acts on it" % disposition],
                )

    def test_a_stale_ruling_fails(self):
        self.rulings.append(self.ruling("export:old:S-9"))
        status, report = self.run_compare()
        self.assertEqual(report["stale_rulings"], ["export:old:S-9"])
        self.assertEqual(status, 1)

    def test_an_unclassified_json_statement_fails(self):
        self.reader_config["scope_exit"] = 4
        self.reader_config["unclassified"] = ["CORE-JSON-9", "CORE-OUT-1"]
        status, _ = self.run_compare()
        self.assertEqual(status, 1)

    def test_a_short_export_set_stops_with_three(self):
        (self.exports / "old.json").unlink()
        status, report = self.run_compare()
        self.assertEqual(status, 3)
        self.assertIsNone(report)

    def test_each_mutant_makes_the_change_its_row_names(self):
        original = sample_export()
        produced = compare.mutate(json.dumps(original).encode("utf-8"))
        self.assertEqual(len(produced), 13)

        def parsed(name):
            return json.loads(produced[name][0].decode("utf-8"))

        def without(member):
            value = sample_export()
            del value[member]
            return value

        def with_first_column_without(member):
            value = sample_export()
            del value["columns"][0][member]
            return value

        def changed(edit):
            value = sample_export()
            edit(value)
            return value

        self.assertEqual(parsed("drop-profile"), without("profile"))
        self.assertEqual(parsed("drop-title"), without("title"))
        self.assertEqual(parsed("drop-columns"), without("columns"))
        self.assertEqual(parsed("columns-not-array"), changed(lambda v: v.update(columns={})))
        self.assertEqual(parsed("column-not-object"), changed(lambda v: v["columns"].__setitem__(0, "column")))
        self.assertEqual(parsed("drop-column-id"), with_first_column_without("id"))
        self.assertEqual(parsed("drop-column-title"), with_first_column_without("title"))
        self.assertEqual(parsed("drop-column-kind"), with_first_column_without("kind"))
        self.assertEqual(parsed("top-level-array"), [original])
        self.assertEqual(parsed("duplicate-column-id"), changed(lambda v: v["columns"][1].__setitem__("id", "a")))
        self.assertEqual(parsed("empty-columns"), changed(lambda v: v.update(columns=[])))
        self.assertEqual(parsed("unprefixed-kind"), changed(lambda v: v["columns"][1].__setitem__("kind", "undeclaredkind")))

        broken = produced["not-utf8"][0]
        self.assertIn(b"\xc3\x28", broken)
        with self.assertRaises(UnicodeDecodeError):
            broken.decode("utf-8")
        self.assertEqual(
            json.loads(broken.replace(b"\xc3\x28", b"@@TITLE@@")),
            changed(lambda v: v.update(title="@@TITLE@@")),
        )
        self.assertEqual(
            [statement for _, (_, statement) in produced.items()],
            ["CORE-JSON-3"] * 3 + ["CORE-JSON-4"] + ["CORE-JSON-5"] * 4 + ["CORE-JSON-2"] * 2
            + ["CORE-STATE-1", "CORE-BENCH-2", "CORE-STATE-11"],
        )


class ReaderImportsTest(unittest.TestCase):
    def test_the_reader_imports_only_the_standard_library_and_its_own_modules(self):
        directory = REPOSITORY / "conformance" / "interchange-reader"
        sources = sorted(p for p in directory.rglob("*.py") if "__pycache__" not in p.parts)
        self.assertGreater(len(sources), 0, "the reader's directory holds no Python source")
        own = {p.stem for p in directory.glob("*.py")} | {"tests"}
        imported = 0
        for source in sources:
            tree = ast.parse(source.read_bytes(), filename=str(source))
            for node in ast.walk(tree):
                if isinstance(node, ast.Import):
                    names = [alias.name for alias in node.names]
                elif isinstance(node, ast.ImportFrom) and node.level == 0:
                    names = [node.module]
                else:
                    continue
                for name in names:
                    imported += 1
                    top = name.split(".")[0]
                    self.assertTrue(
                        top in sys.stdlib_module_names or top in own,
                        "%s imports %s, which is neither the standard library nor a module of the reader"
                        % (source.relative_to(REPOSITORY).as_posix(), name),
                    )
        self.assertGreater(imported, 0, "no import statement was read")


if __name__ == "__main__":
    unittest.main()
