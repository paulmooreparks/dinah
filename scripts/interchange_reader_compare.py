#!/usr/bin/env python3
"""Compare the independent reader of the interchange form with Dinah.

The reader under conformance/interchange-reader/ was written from the profile
alone by an author who never saw this repository's code. This script asks it
and Dinah the same questions and fails when they answer differently, until a
person has ruled which side is wrong in the rulings file.

    python scripts/interchange_reader_compare.py --dinah <binary>
        --exports <directory of <fixture>.json> --conformance <conformance.json>
        --rulings conformance/interchange-reader-rulings.json --out <directory>

Three sets of questions are asked. The reader judges the export Dinah writes
for every compatibility fixture. Dinah is handed every fixture the reader's
author wrote, through init --from, and its verdict is set beside the one that
author expected. And both sides judge thirteen broken variants of one real
export, each breaking one MUST statement, which both are expected to refuse.

A leak scan then looks for member names Dinah writes that the profile never
names, quoted in any file the reader's author wrote. Such a name could only
have come from outside the profile.

Exit status is 0 when every disagreement is ruled and no ruling is stale, 1 on
an unruled disagreement, a stale ruling, a reader that contradicts its own
expectations, or a CORE-JSON statement the reader leaves unclassified, 2 on a
usage error, and 3 when a set this script depends on is empty or short or a
command could not run.

--root names the repository root and defaults to the one holding this script.
It exists so the tests can run the script against a tree of stubs.
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

READER_DIR = Path("conformance") / "interchange-reader"
PROFILE = Path("docs") / "spec" / "core-profile.md"
MANIFEST = Path("internal") / "bench" / "testdata" / "compat" / "manifest.json"

# The files of the reader's directory that its author did not write, and which
# the leak scan therefore leaves alone.
NOT_THE_AUTHORS = {"BRIEF.md", "README.md", "provenance.json"}

DISPOSITIONS = {
    "tool-defect",
    "profile-defect",
    "reader-defect",
    "both-conform",
    "not-a-leak",
}

BACKTICKED = re.compile(r"`([^`\n]+)`")

TITLE_PLACEHOLDER = "@@TITLE@@"


class Stop(Exception):
    """A sweep came up short or a command could not run; exit 3."""


def serialise(value):
    return json.dumps(value, ensure_ascii=False, indent=2).encode("utf-8")


def first_column(obj):
    return obj["columns"][0]


def second_column(obj):
    return obj["columns"][1]


def _drop(member):
    def change(obj):
        del obj[member]
        return serialise(obj)

    return change


def _drop_from_first_column(member):
    def change(obj):
        del first_column(obj)[member]
        return serialise(obj)

    return change


def _replace_columns(value):
    def change(obj):
        obj["columns"] = value
        return serialise(obj)

    return change


def _column_not_object(obj):
    obj["columns"][0] = "column"
    return serialise(obj)


def _top_level_array(obj):
    return serialise([obj])


def _not_utf8(obj):
    obj["title"] = TITLE_PLACEHOLDER
    return serialise(obj).replace(TITLE_PLACEHOLDER.encode("utf-8"), b"\xc3\x28")


def _duplicate_column_id(obj):
    second_column(obj)["id"] = first_column(obj)["id"]
    return serialise(obj)


def _unprefixed_kind(obj):
    second_column(obj)["kind"] = "undeclaredkind"
    return serialise(obj)


# Each mutant: its name, the function producing its bytes from a fresh copy of
# the sample export, and the one statement it breaks.
MUTANTS = (
    ("drop-profile", _drop("profile"), "CORE-JSON-3"),
    ("drop-title", _drop("title"), "CORE-JSON-3"),
    ("drop-columns", _drop("columns"), "CORE-JSON-3"),
    ("columns-not-array", _replace_columns({}), "CORE-JSON-4"),
    ("column-not-object", _column_not_object, "CORE-JSON-5"),
    ("drop-column-id", _drop_from_first_column("id"), "CORE-JSON-5"),
    ("drop-column-title", _drop_from_first_column("title"), "CORE-JSON-5"),
    ("drop-column-kind", _drop_from_first_column("kind"), "CORE-JSON-5"),
    ("top-level-array", _top_level_array, "CORE-JSON-2"),
    ("not-utf8", _not_utf8, "CORE-JSON-2"),
    ("duplicate-column-id", _duplicate_column_id, "CORE-STATE-1"),
    ("empty-columns", _replace_columns([]), "CORE-BENCH-2"),
    ("unprefixed-kind", _unprefixed_kind, "CORE-STATE-11"),
)


def mutate(sample_bytes):
    """Return {name: (bytes, statement)} for the thirteen mutants."""
    produced = {}
    for name, change, statement in MUTANTS:
        produced[name] = (change(json.loads(sample_bytes)), statement)
    return produced


class Comparison:
    def __init__(self, args):
        self.root = Path(args.root).resolve()
        self.reader_dir = self.root / READER_DIR
        self.reader = self.reader_dir / "reader.py"
        self.profile = self.root / PROFILE
        self.manifest = self.root / MANIFEST
        self.dinah = Path(args.dinah).resolve()
        self.exports = Path(args.exports).resolve()
        self.conformance_path = Path(args.conformance).resolve()
        self.rulings_path = Path(args.rulings).resolve()
        self.out = Path(args.out).resolve()
        self.counts = {}
        self.disagreements = []
        self.self_inconsistent = []
        self.stats = {}
        self.log = []

    # Running the two sides.

    def say(self, line):
        print(line)
        self.log.append(line)

    def run_reader(self, *arguments):
        """Run the reader and return (exit status, parsed standard output)."""
        done = subprocess.run(
            [sys.executable, str(self.reader), "--profile", str(self.profile), *arguments],
            capture_output=True,
            check=False,
        )
        if done.returncode not in (0, 4):
            raise Stop(
                "the reader exited %d on %s: %s"
                % (
                    done.returncode,
                    arguments[0],
                    done.stderr.decode("utf-8", "replace").strip()[-2000:],
                )
            )
        try:
            return done.returncode, json.loads(done.stdout.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as err:
            raise Stop("the reader wrote no JSON document on %s: %s" % (arguments[0], err))

    def dinah_command(self):
        if self.dinah.suffix == ".py":
            return [sys.executable, str(self.dinah)]
        return [str(self.dinah)]

    def run_dinah(self, cwd, *arguments):
        env = dict(os.environ)
        env.update(
            {
                "DINAH_HOME": str(self.out / "home"),
                "DINAH_ACTOR": "reader",
                "DINAH_WORKBENCH": "",
                "DINAH_LANG": "",
            }
        )
        cwd.mkdir(parents=True, exist_ok=True)
        return subprocess.run(
            self.dinah_command() + ["--format", "json", *arguments],
            cwd=str(cwd),
            env=env,
            capture_output=True,
            check=False,
        )

    def ask_dinah(self, label, source):
        """Hand source to init --from, then export what it wrote.

        Returns (verdict, refusal, re-export path or None), where the verdict is
        accept, refuse or accept-unreadable.
        """
        target = self.out / "work" / label
        if target.exists():
            shutil.rmtree(target)
        done = self.run_dinah(
            self.out / "work",
            "init",
            str(target),
            "--from",
            str(Path(source).resolve()),
            "--slug",
            "ir",
            "--operator",
            "reader",
        )
        if done.returncode == 2:
            try:
                refusal = json.loads(done.stdout.decode("utf-8")).get("refusal")
            except (UnicodeDecodeError, json.JSONDecodeError, AttributeError):
                refusal = None
            return "refuse", refusal, None
        if done.returncode != 0:
            raise Stop(
                "dinah init --from %s exited %d: %s"
                % (source, done.returncode, done.stderr.decode("utf-8", "replace").strip())
            )
        found = sorted((target / ".dinah").glob("*"))
        if len(found) != 1:
            raise Stop("dinah init wrote %d workbenches under %s, expected one" % (len(found), target))
        exported = self.run_dinah(self.out / "work", "--workbench", str(found[0]), "export")
        if exported.returncode == 2:
            return "accept-unreadable", None, None
        if exported.returncode != 0:
            raise Stop(
                "dinah export of %s exited %d: %s"
                % (label, exported.returncode, exported.stderr.decode("utf-8", "replace").strip())
            )
        path = self.out / "work" / (label + ".export.json")
        path.write_bytes(exported.stdout)
        return "accept", None, path

    # Recording.

    def stat(self, statement, group, key):
        row = self.stats.setdefault(statement, {})
        counts = row.setdefault(group, {})
        counts[key] = counts.get(key, 0) + 1

    def disagree(self, source, subject, statement, reader, dinah):
        key = "%s:%s:%s" % (source, subject, statement)
        self.disagreements.append(
            {
                "key": key,
                "source": source,
                "subject": subject,
                "statement": statement,
                "reader": reader,
                "dinah": dinah,
                "ruling": None,
            }
        )

    def count(self, name, value):
        self.counts[name] = value
        self.say("%s: %d" % (name, value))

    # The sweeps.

    def load_sets(self):
        manifest = json.loads(self.manifest.read_text(encoding="utf-8"))
        self.fixture_dirs = [entry["directory"] for entry in manifest.get("fixtures", [])]
        self.sample = [e["directory"] for e in manifest.get("fixtures", []) if e.get("sample")]
        self.count("manifest_fixtures", len(self.fixture_dirs))
        if not self.fixture_dirs:
            raise Stop("the fixture manifest lists no fixture")
        if len(self.sample) != 1:
            raise Stop("the fixture manifest marks %d fixtures as the sample, expected one" % len(self.sample))

        export_files = sorted(self.exports.glob("*.json")) if self.exports.is_dir() else []
        self.count("exports", len(export_files))
        names = {p.stem for p in export_files}
        missing = [d for d in self.fixture_dirs if d not in names]
        if len(export_files) != len(self.fixture_dirs) or missing:
            raise Stop(
                "%d export files for %d manifest fixtures; missing %s"
                % (len(export_files), len(self.fixture_dirs), missing)
            )

        expectations = json.loads(
            (self.reader_dir / "fixtures" / "expectations.json").read_text(encoding="utf-8")
        )
        self.expectations = expectations.get("fixtures", [])
        self.count("reader_fixtures", len(self.expectations))
        verdicts = {e.get("expect") for e in self.expectations}
        if not {"accept", "refuse"} <= verdicts:
            raise Stop("the reader's fixtures need at least one expected accept and one expected refuse")

        self.conformance = json.loads(self.conformance_path.read_text(encoding="utf-8"))
        self.count("conformance_statements", len(self.conformance.get("statements", [])))
        if not self.conformance.get("statements"):
            raise Stop("the conformance report carries no statement")

    def check_exports(self):
        files = [self.exports / (d + ".json") for d in self.fixture_dirs]
        _, answer = self.run_reader("check", *map(str, files))
        self.reader_identity = answer.get("reader")
        results = answer.get("results", [])
        if len(results) != len(files):
            raise Stop("the reader returned %d results for %d exports" % (len(results), len(files)))
        for directory, result in zip(self.fixture_dirs, results):
            failing = []
            for statement in result.get("statements", []):
                outcome = statement.get("result")
                self.stat(statement.get("id"), "exports", outcome.replace("-", "_") if outcome else "missing")
                if outcome == "fail":
                    failing.append(statement)
                    self.disagree(
                        "export",
                        directory,
                        statement.get("id"),
                        statement.get("detail"),
                        "Dinah exported this object",
                    )
            if result.get("verdict") == "refuse" and not failing:
                self.disagree(
                    "export",
                    directory,
                    "verdict",
                    "refuse, naming no failing statement",
                    "Dinah exported this object",
                )

    def check_reader_fixtures(self):
        directory = self.reader_dir / "fixtures"
        files = [directory / e["file"] for e in self.expectations]
        _, answer = self.run_reader("check", *map(str, files))
        results = answer.get("results", [])
        if len(results) != len(files):
            raise Stop("the reader returned %d results for %d of its fixtures" % (len(results), len(files)))
        for n, (expectation, result) in enumerate(zip(self.expectations, results), start=1):
            name = expectation["file"]
            expect = expectation.get("expect")
            named = expectation.get("refusal")
            if result.get("verdict") != expect or (
                named is not None and result.get("refusal") != named
            ):
                self.self_inconsistent.append(
                    {
                        "key": "self:%s" % name,
                        "expected": {"verdict": expect, "refusal": named},
                        "reader": {"verdict": result.get("verdict"), "refusal": result.get("refusal")},
                    }
                )
            verdict, refusal, reexport = self.ask_dinah("fixture-%d" % n, directory / name)
            agrees = (
                verdict == expect
                and not (expect == "refuse" and named is not None and refusal != named)
            )
            for statement in expectation.get("statements", []):
                self.stat(statement, "fixtures", "agree" if agrees else "disagree")
            if not agrees:
                self.disagree(
                    "fixture",
                    name,
                    "verdict",
                    "%s%s" % (expect, " (%s)" % named if named else ""),
                    "%s%s" % (verdict, " (%s)" % refusal if refusal else ""),
                )
            if verdict == "accept":
                _, paired = self.run_reader("pair", str(directory / name), str(reexport))
                for statement in paired.get("result", {}).get("statements", []):
                    outcome = statement.get("result")
                    self.stat(statement.get("id"), "pairs", outcome.replace("-", "_") if outcome else "missing")
                    if outcome == "fail":
                        self.disagree(
                            "pair",
                            name,
                            statement.get("id"),
                            statement.get("detail"),
                            "Dinah admitted the object and wrote this back",
                        )

    def check_mutants(self):
        sample = self.exports / (self.sample[0] + ".json")
        produced = mutate(sample.read_bytes())
        directory = self.out / "mutants"
        directory.mkdir(parents=True, exist_ok=True)
        paths = []
        for name, (data, _) in produced.items():
            path = directory / (name + ".json")
            path.write_bytes(data)
            paths.append(path)
        self.count("mutants", len(paths))
        if len(paths) != 13:
            raise Stop("%d mutants were written, expected 13" % len(paths))
        _, answer = self.run_reader("check", *map(str, paths))
        results = answer.get("results", [])
        if len(results) != len(paths):
            raise Stop("the reader returned %d results for %d mutants" % (len(results), len(paths)))
        self.mutant_rows = []
        for (name, (_, statement)), path, result in zip(produced.items(), paths, results):
            failed = [s.get("id") for s in result.get("statements", []) if s.get("result") == "fail"]
            verdict, refusal, _ = self.ask_dinah("mutant-" + name, path)
            reader_agrees = statement in failed
            dinah_agrees = verdict == "refuse"
            self.stat(statement, "mutants", "agree" if reader_agrees and dinah_agrees else "disagree")
            self.mutant_rows.append(
                {
                    "name": name,
                    "statement": statement,
                    "reader": {"verdict": result.get("verdict"), "refusal": result.get("refusal"), "failed": failed},
                    "dinah": {"verdict": verdict, "refusal": refusal},
                }
            )
            if not reader_agrees:
                self.disagree(
                    "mutant", name, "reader",
                    "%s, failing %s" % (result.get("verdict"), failed or "nothing"),
                    "the mutant breaks %s" % statement,
                )
            if not dinah_agrees:
                self.disagree(
                    "mutant", name, "dinah",
                    "the mutant breaks %s" % statement,
                    verdict,
                )

    def check_scope(self):
        status, answer = self.run_reader("scope")
        self.unclassified = answer.get("unclassified", [])
        self.stale_scope = answer.get("stale", [])
        self.scope_failure = None
        if status == 4:
            names = [s for s in self.unclassified if s.startswith("CORE-JSON-")]
            self.scope_failure = "the reader leaves these CORE-JSON statements unclassified: %s" % (
                ", ".join(names) or "(the reader named none)"
            )
        scope = json.loads((self.reader_dir / "scope.json").read_text(encoding="utf-8"))
        self.scope = {k: v.get("scope") for k, v in scope.get("statements", {}).items()}

    def leak_scan(self):
        names = set()
        for path in sorted(self.exports.glob("*.json")):
            obj = json.loads(path.read_bytes())
            if not isinstance(obj, dict):
                continue
            names.update(obj)
            for column in obj.get("columns", []) if isinstance(obj.get("columns"), list) else []:
                if isinstance(column, dict):
                    names.update(column)
        profile_tokens = set(BACKTICKED.findall(self.profile.read_text(encoding="utf-8")))
        self.leak_names = sorted(names - profile_tokens)
        self.count("leak_names", len(self.leak_names))
        scanned = 0
        for path in sorted(self.reader_dir.rglob("*")):
            relative = path.relative_to(self.reader_dir).as_posix()
            if not path.is_file() or relative in NOT_THE_AUTHORS or "__pycache__" in path.parts:
                continue
            scanned += 1
            text = path.read_bytes().decode("utf-8", "replace")
            for number, line in enumerate(text.splitlines(), start=1):
                for name in self.leak_names:
                    if '"%s"' % name in line or "'%s'" % name in line:
                        self.disagree(
                            "leak",
                            "%s:%d" % (relative, number),
                            name,
                            line.strip()[:200],
                            "Dinah writes %s and the profile never names it" % name,
                        )
        self.count("leak_scanned_files", scanned)

    def apply_rulings(self):
        rulings = json.loads(self.rulings_path.read_text(encoding="utf-8")).get("rulings", [])
        by_key = {}
        for ruling in rulings:
            by_key.setdefault(ruling.get("key"), ruling)
        present = {d["key"] for d in self.disagreements}
        self.unruled = []
        for d in self.disagreements:
            ruling = by_key.get(d["key"])
            d["ruling"] = ruling
            if ruling is None:
                self.unruled.append((d["key"], "no ruling"))
            elif not ruling.get("ruled_by"):
                self.unruled.append((d["key"], "the ruling's ruled_by is empty"))
            elif ruling.get("disposition") not in DISPOSITIONS:
                self.unruled.append((d["key"], "the disposition %r is not one of the five" % ruling.get("disposition")))
            elif ruling.get("disposition") == "not-a-leak" and not d["key"].startswith("leak:"):
                self.unruled.append((d["key"], "not-a-leak rules only a leak-scan key"))
        self.stale = sorted(k for k in by_key if k not in present)
        self.counts["disagreements"] = len(self.disagreements)
        self.counts["unruled"] = len(self.unruled)
        self.counts["ruled"] = len(self.disagreements) - len(self.unruled)
        self.counts["stale_rulings"] = len(self.stale)
        self.counts["self_inconsistent"] = len(self.self_inconsistent)

    # The report.

    def statement_rows(self):
        order = [s["id"] for s in self.conformance["statements"]]
        suite = {s["id"]: s for s in self.conformance["statements"]}
        chosen = [i for i in order if self.scope.get(i) in ("export", "pair")]
        chosen += sorted(i for i, s in self.scope.items() if s in ("export", "pair") and i not in suite)
        rows = []
        for statement in chosen:
            conformance = suite.get(statement)
            stats = self.stats.get(statement, {})
            rows.append(
                {
                    "id": statement,
                    "suite": {
                        "tests": (conformance or {}).get("tests") or None,
                        "out_of_reach": (conformance or {}).get("out_of_reach"),
                    },
                    "reader_scope": self.scope.get(statement),
                    "exports": {k: stats.get("exports", {}).get(k, 0) for k in ("pass", "fail", "not_applicable")},
                    "fixtures": {k: stats.get("fixtures", {}).get(k, 0) for k in ("agree", "disagree")},
                    "pairs": {k: stats.get("pairs", {}).get(k, 0) for k in ("pass", "fail", "not_applicable")},
                    "mutants": {k: stats.get("mutants", {}).get(k, 0) for k in ("agree", "disagree")},
                }
            )
        return rows

    def provenance(self):
        path = self.reader_dir / "provenance.json"
        return json.loads(path.read_text(encoding="utf-8")) if path.exists() else None

    def write_report(self):
        rows = self.statement_rows()
        report = {
            "profile": self.conformance.get("profile"),
            "reader": self.reader_identity,
            "provenance": self.provenance(),
            "counts": self.counts,
            "statements": rows,
            "disagreements": self.disagreements,
            "self_inconsistent": self.self_inconsistent,
            "unruled": [{"key": k, "why": why} for k, why in self.unruled],
            "stale_rulings": self.stale,
            "unclassified": self.unclassified,
            "stale_scope": self.stale_scope,
            "mutants": self.mutant_rows,
            "leak_names": self.leak_names,
        }
        self.out.mkdir(parents=True, exist_ok=True)
        (self.out / "report.json").write_text(
            json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8", newline="\n"
        )
        (self.out / "report.md").write_text(self.markdown(report), encoding="utf-8", newline="\n")

    def markdown(self, report):
        c = report["counts"]
        first = (
            "The independent reader and Dinah disagree %d times, %d of them unruled, "
            "with %d stale rulings and %d fixtures the reader judges against its own expectation."
            % (c["disagreements"], c["unruled"], c["stale_rulings"], c["self_inconsistent"])
        )
        if not report["leak_names"]:
            first += " The leak scan was vacuous, because Dinah writes no member name the profile does not name."
        lines = [first, "", "## Counts", "", "| Set | Count |", "| --- | --- |"]
        lines += ["| %s | %d |" % (k, v) for k, v in c.items()]
        lines += [
            "",
            "## Statements",
            "",
            "| Statement | Suite | Reader scope | Exports | Fixtures | Pairs | Mutants |",
            "| --- | --- | --- | --- | --- | --- | --- |",
        ]
        for row in report["statements"]:
            suite = row["suite"]
            suite_text = ", ".join(suite["tests"] or []) or (
                "out of reach" if suite["out_of_reach"] else "neither"
            )
            lines.append(
                "| %s | %s | %s | %s | %s | %s | %s |"
                % (
                    row["id"],
                    suite_text,
                    row["reader_scope"],
                    "%(pass)d pass, %(fail)d fail, %(not_applicable)d n/a" % row["exports"],
                    "%(agree)d agree, %(disagree)d disagree" % row["fixtures"],
                    "%(pass)d pass, %(fail)d fail, %(not_applicable)d n/a" % row["pairs"],
                    "%(agree)d agree, %(disagree)d disagree" % row["mutants"],
                )
            )
        lines += ["", "## Disagreements", ""]
        if not report["disagreements"]:
            lines.append("There were none.")
        for d in report["disagreements"]:
            ruling = d["ruling"]
            state = (
                "ruled %s by %s" % (ruling.get("disposition"), ruling.get("ruled_by"))
                if ruling and ruling.get("ruled_by")
                else "unruled"
            )
            lines.append("- `%s` is %s. Reader: %s Dinah: %s" % (d["key"], state, d["reader"], d["dinah"]))
        for s in report["self_inconsistent"]:
            lines.append("- `%s`: the reader contradicts its own expectation, which no ruling covers." % s["key"])
        for k in report["stale_rulings"]:
            lines.append("- `%s` is ruled and matches no disagreement of this run." % k)
        lines += ["", "## Leak scan", ""]
        lines.append(
            "The scan looked for %d member names: %s."
            % (len(report["leak_names"]), ", ".join("`%s`" % n for n in report["leak_names"]) or "none")
        )
        return "\n".join(lines) + "\n"

    def run(self):
        self.load_sets()
        self.check_exports()
        self.check_reader_fixtures()
        self.check_mutants()
        self.check_scope()
        self.leak_scan()
        self.apply_rulings()
        self.write_report()
        failures = []
        failures += ["%s: %s" % (k, why) for k, why in self.unruled]
        failures += ["%s: a stale ruling" % k for k in self.stale]
        failures += ["%s: the reader contradicts its own expectation" % s["key"] for s in self.self_inconsistent]
        if self.scope_failure:
            failures.append(self.scope_failure)
        for line in failures:
            print(line, file=sys.stderr)
        self.say(
            "%d disagreements, %d unruled, %d stale rulings, %d self-inconsistencies"
            % (len(self.disagreements), len(self.unruled), len(self.stale), len(self.self_inconsistent))
        )
        return 1 if failures else 0


def main(argv=None):
    parser = argparse.ArgumentParser(description="Compare the independent reader with Dinah.")
    parser.add_argument("--dinah", required=True)
    parser.add_argument("--exports", required=True)
    parser.add_argument("--conformance", required=True)
    parser.add_argument("--rulings", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--root", default=str(ROOT))
    args = parser.parse_args(argv)
    comparison = Comparison(args)
    try:
        return comparison.run()
    except Stop as stop:
        print("stopped: %s" % stop, file=sys.stderr)
        return 3
    except (OSError, ValueError, KeyError) as err:
        print("stopped: %s" % err, file=sys.stderr)
        return 3


if __name__ == "__main__":
    sys.exit(main())
