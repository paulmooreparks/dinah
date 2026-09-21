"""A reader of the interchange form that section 5.7 of the core profile defines.

    python reader.py --profile <path> check <file>...
    python reader.py --profile <path> pair <sent> <returned>
    python reader.py --profile <path> scope

The profile is read at run time: the statements come from the expression of
section 3.2 and the version from the profile's opening lines. Which statements
are judged comes from scope.json beside this file. Every place the profile
admitted more than one reading cites the entry of READINGS.md (R-n) that
records the choice.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from decimal import Decimal
from pathlib import Path

READER_NAME = "interchange-reader"
SCOPE_PATH = Path(__file__).resolve().parent / "scope.json"
SCOPES = ("export", "pair", "none")

EXIT_OK = 0
EXIT_INTERNAL = 1
EXIT_USAGE = 2
EXIT_PROFILE = 3
EXIT_UNCLASSIFIED = 4

# Section 3.2, applied to every line of the profile (R-2).
STATEMENT_RE = re.compile(r"^\[([A-Z][A-Z0-9]*(-[A-Z0-9]+)*)\] (.+)$")
# The opening lines state the version identity (R-1).
IDENTITY_RE = re.compile(r"Version identity: `([^`]+)`")
IDENTITY_PARTS_RE = re.compile(r"(\S+) ([0-9]+)\.([0-9]+)")
OPENING_LINES = 10

# Section 5.7 member names. CORE-JSON-3 and CORE-JSON-5 require the first
# three of each level; CORE-JSON-10 to CORE-JSON-13 permit the rest.
REQUIRED_TOP = ("profile", "title", "columns")
OPTIONAL_TOP = ("fields", "field_values", "tiers")
REQUIRED_COLUMN = ("id", "title", "kind")
JSON10_COLUMN = ("instructions", "operator_owned", "capacity", "slug", "gate_items")
JSON12_COLUMN = ("field_values", "require_fields")
TOP_MEMBERS = REQUIRED_TOP + OPTIONAL_TOP
COLUMN_MEMBERS = REQUIRED_COLUMN + JSON10_COLUMN + JSON12_COLUMN

# Section 5.2.
DECLARED_KINDS = ("intake", "work", "done")

# Section 6.1: the refusal names reading an interchange object can produce,
# in the order the checks ahead of every list put them (R-17).
REFUSAL_BY_STATEMENT = {"CORE-BENCH-5": "unsupported-version"}
DEFAULT_REFUSAL = "malformed"
REFUSAL_ORDER = ("unsupported-version", "malformed")

PASS, FAIL, NA = "pass", "fail", "not-applicable"

NO_OBJECT = "No JSON object was read, so this statement was not judged; CORE-JSON-2 reports why."
NOT_IMPLEMENTED = "The reader implements no judgement for this statement."


def diagnose(message: str) -> None:
    print(f"reader: {message}", file=sys.stderr)


# ---------------------------------------------------------------------------
# The profile


class ProfileError(Exception):
    pass


class Profile:
    def __init__(self, sha256, identity, family, version, statements):
        self.sha256 = sha256
        self.identity = identity          # "dinah-core 0.17"
        self.family = family              # "dinah-core"
        self.version = version            # (0, 17)
        self.statements = statements      # [(identifier, text)] in published order

    @property
    def identifiers(self):
        return [ident for ident, _ in self.statements]

    @classmethod
    def load(cls, path):
        try:
            data = Path(path).read_bytes()
        except OSError as exc:
            raise ProfileError(f"the profile {path} could not be read: {exc}") from None
        try:
            text = data.decode("utf-8")
        except UnicodeDecodeError as exc:
            raise ProfileError(
                f"the profile {path} is not UTF-8 ({exc.reason} at byte {exc.start})") from None
        if text.startswith("\ufeff"):
            text = text[1:]
        lines = [line[:-1] if line.endswith("\r") else line for line in text.split("\n")]

        statements, seen = [], set()
        for line in lines:
            match = STATEMENT_RE.match(line)
            if not match:
                continue
            ident = match.group(1)
            if ident in seen:
                diagnose(f"the profile publishes {ident} more than once; the first is kept")
                continue
            seen.add(ident)
            statements.append((ident, match.group(3)))
        if not statements:
            raise ProfileError(f"the profile {path} yielded no statements under section 3.2")

        identity = None
        for line in lines[:OPENING_LINES]:
            match = IDENTITY_RE.search(line)
            if match:
                identity = match.group(1)
                break
        if identity is None:
            raise ProfileError(f"the profile {path} states no version identity in its opening lines")
        parts = IDENTITY_PARTS_RE.fullmatch(identity)
        if parts is None:
            raise ProfileError(f"the version identity {identity!r} names no major and minor number")
        version = (int(parts.group(2)), int(parts.group(3)))
        return cls(hashlib.sha256(data).hexdigest(), identity, parts.group(1), version, statements)

    def reader_member(self):
        return {"name": READER_NAME, "profile": self.identity, "profile_sha256": self.sha256}


# ---------------------------------------------------------------------------
# scope.json


class ScopeError(Exception):
    pass


def load_scope(path=SCOPE_PATH):
    """Returns the identity scope.json names and a map of identifier to scope."""
    try:
        document = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise ScopeError(f"{path} could not be read: {exc}") from None
    if not isinstance(document, dict) or not isinstance(document.get("statements"), dict):
        raise ScopeError(f"{path} carries no statements object")
    scopes = {}
    for ident, entry in document["statements"].items():
        scopes[ident] = entry.get("scope") if isinstance(entry, dict) else None
    return document.get("profile"), scopes


# ---------------------------------------------------------------------------
# Reading one file


def _reject_constant(name):
    raise ValueError(f"{name} is not a JSON value")


class Document:
    """What the reader made of one file's bytes."""

    def __init__(self, data: bytes):
        self.utf8 = False
        self.bom = False
        self.parsed = False
        self.value = None
        self.error = None
        self.duplicates = []
        try:
            text = data.decode("utf-8")
        except UnicodeDecodeError as exc:
            self.error = f"the byte at offset {exc.start} is invalid ({exc.reason})"
            return
        self.utf8 = True
        if text.startswith("\ufeff"):
            self.bom = True       # R-15
            text = text[1:]
        try:
            # RFC 8259 grammar: no NaN or Infinity, no trailing text, and no
            # raw control characters in strings (R-16).
            self.value = json.loads(text, object_pairs_hook=self._object,
                                    parse_float=Decimal, parse_constant=_reject_constant)
        except (ValueError, RecursionError, ArithmeticError) as exc:
            self.error = str(exc) or type(exc).__name__
            return
        self.parsed = True

    def _object(self, pairs):
        # Repeated names keep the last value (R-16).
        obj = {}
        for name, value in pairs:
            if name in obj:
                self.duplicates.append(name)
            obj[name] = value
        return obj

    @property
    def object(self):
        if self.parsed and isinstance(self.value, dict):
            return self.value
        return None


# ---------------------------------------------------------------------------
# Small helpers for judgements and their sentences


def json_equal(a, b):
    """Equality of two JSON values that keeps true apart from 1."""
    if isinstance(a, bool) or isinstance(b, bool):
        return type(a) is type(b) and a == b
    if isinstance(a, (int, Decimal)) and isinstance(b, (int, Decimal)):
        return a == b
    if isinstance(a, str) or isinstance(b, str):
        return isinstance(a, str) and isinstance(b, str) and a == b
    if a is None or b is None:
        return a is None and b is None
    if isinstance(a, list) and isinstance(b, list):
        return len(a) == len(b) and all(json_equal(x, y) for x, y in zip(a, b))
    if isinstance(a, dict) and isinstance(b, dict):
        return a.keys() == b.keys() and all(json_equal(a[k], b[k]) for k in a)
    return False


def json_kind(value):
    if isinstance(value, dict):
        return "a JSON object"
    if isinstance(value, list):
        return "a JSON array"
    if isinstance(value, str):
        return "a JSON string"
    if isinstance(value, bool):
        return "the literal " + ("true" if value else "false")
    if value is None:
        return "the literal null"
    return "a JSON number"


def show(value):
    text = json.dumps(value, default=float)
    return text if len(text) <= 40 else text[:37] + "..."


def names(items, conjunction="and"):
    items = [str(item) for item in items]
    if len(items) == 1:
        return items[0]
    return ", ".join(items[:-1]) + f" {conjunction} " + items[-1]


def positions(indices):
    word = "column" if len(indices) == 1 else "columns"
    return f"{word} {names([i + 1 for i in indices])}"


def plural(count, word):
    return f"{count} {word}" + ("" if count == 1 else "s")


def is_title(value):
    """R-4: a JSON string carrying something other than white space."""
    return isinstance(value, str) and value.strip() != ""


def is_identifier(value):
    """R-5: a non-empty JSON string, compared exactly."""
    return isinstance(value, str) and value != ""


def is_kind(value):
    """R-9: a declared kind spelled exactly, or a name containing a full stop."""
    return isinstance(value, str) and (value in DECLARED_KINDS or "." in value)


def is_capacity(value):
    """R-10: a JSON number whose value is a whole number greater than zero."""
    if isinstance(value, bool):
        return False
    if isinstance(value, int):
        return value > 0
    if isinstance(value, Decimal):
        return value > 0 and value == value.to_integral_value()
    return False


def declared_version(value, family):
    """R-6: the identity string, a solidus or one space, then major.minor."""
    if not isinstance(value, str):
        return None
    match = re.fullmatch(re.escape(family) + r"[/ ]([0-9]+)\.([0-9]+)", value)
    if match is None:
        return None
    return int(match.group(1)), int(match.group(2))


def dotted(version):
    return f"{version[0]}.{version[1]}"


# ---------------------------------------------------------------------------
# Judgements of one interchange object (scope "export")


class Subject:
    def __init__(self, doc: Document, profile: Profile):
        self.doc = doc
        self.profile = profile
        self.obj = doc.object

    @property
    def columns(self):
        value = self.obj.get("columns") if self.obj is not None else None
        return value if isinstance(value, list) else None

    def column_objects(self):
        return [(i, c) for i, c in enumerate(self.columns or []) if isinstance(c, dict)]

    def columns_carrying(self, member):
        return [(i, c[member]) for i, c in self.column_objects() if member in c]


EXPORT_JUDGES = {}


def export(ident, needs_object=True):
    def register(judge):
        def run(subject):
            if needs_object and subject.obj is None:
                return NA, NO_OBJECT
            return judge(subject)
        EXPORT_JUDGES[ident] = run
        return judge
    return register


@export("CORE-JSON-2", needs_object=False)
def _json_2(s):
    doc = s.doc
    if not doc.utf8:
        return FAIL, f"The file's bytes are not valid UTF-8: {doc.error}."
    if not doc.parsed:
        return FAIL, f"The file's text is not a JSON text under RFC 8259: {doc.error}."
    if not isinstance(doc.value, dict):
        return FAIL, f"The JSON text is {json_kind(doc.value)} rather than one JSON object."
    notes = []
    if doc.bom:
        notes.append("begins with a byte order mark, which the reader ignored as RFC 8259 permits")
    if doc.duplicates:
        notes.append(f"repeats the member name {show(doc.duplicates[0])}, "
                     "of which the reader judged the last value")
    if notes:
        return PASS, "The file is one JSON object encoded in UTF-8, and it " + " and ".join(notes) + "."
    return PASS, "The file is one JSON object encoded in UTF-8."


@export("CORE-JSON-3")
def _json_3(s):
    missing = [m for m in REQUIRED_TOP if m not in s.obj]
    if missing:
        word = "member" if len(missing) == 1 else "members"
        return FAIL, f"The object does not carry the {word} {names(missing)}."
    return PASS, "The object carries the members profile, title and columns."


@export("CORE-BENCH-1")
def _bench_1(s):
    if "title" not in s.obj:
        return NA, "The object carries no title member, whose absence CORE-JSON-3 reports."
    if is_title(s.obj["title"]):
        return PASS, "The object's title member carries a title."
    return FAIL, f"The object's title member is {show(s.obj['title'])}, which is not a string carrying a name."


@export("CORE-BENCH-2")
def _bench_2(s):
    columns = s.columns
    if columns is None:
        return NA, "The object carries no columns array, which CORE-JSON-3 or CORE-JSON-4 reports."
    if not columns:
        return FAIL, "The columns array is empty, so the definition carries no column."
    return PASS, f"The columns array carries {plural(len(columns), 'element')}."


@export("CORE-BENCH-3")
def _bench_3(s):
    if "profile" not in s.obj:
        return NA, "The object carries no profile member, whose absence CORE-JSON-3 reports."
    version = declared_version(s.obj["profile"], s.profile.family)
    if version is None:
        return FAIL, (f"The profile member is {show(s.obj['profile'])}, which does not declare a major "
                      f"and a minor number of {s.profile.family}.")
    return PASS, f"The profile member declares version {dotted(version)} of {s.profile.family}."


@export("CORE-BENCH-5")
def _bench_5(s):
    version = declared_version(s.obj.get("profile"), s.profile.family)
    if version is None:
        return NA, "The object declares no readable profile version, which CORE-JSON-3 or CORE-BENCH-3 reports."
    claim = dotted(s.profile.version)
    if version > s.profile.version:
        return FAIL, (f"The object declares version {dotted(version)}, which sorts after {claim}, "
                      "the version a tool conforming to this profile claims.")
    return PASS, f"The object declares version {dotted(version)}, which does not sort after {claim}."


@export("CORE-JSON-4")
def _json_4(s):
    if "columns" not in s.obj:
        return NA, "The object carries no columns member, whose absence CORE-JSON-3 reports."
    if not isinstance(s.obj["columns"], list):
        return FAIL, f"The columns member is {json_kind(s.obj['columns'])} rather than a JSON array."
    return PASS, "The columns member is a JSON array, whose element order the reader takes as the flow."


@export("CORE-JSON-5")
def _json_5(s):
    columns = s.columns
    if columns is None:
        return NA, "The object carries no columns array whose elements could be judged."
    if not columns:
        return NA, "The columns array has no element to judge."
    problems = []
    not_objects = [i for i, c in enumerate(columns) if not isinstance(c, dict)]
    if not_objects:
        verb = "is" if len(not_objects) == 1 else "are"
        problems.append(f"{positions(not_objects)} {verb} not a JSON object")
    for i, column in s.column_objects():
        missing = [m for m in REQUIRED_COLUMN if m not in column]
        if missing:
            problems.append(f"column {i + 1} does not carry {names(missing)}")
    if problems:
        return FAIL, "In the columns array, " + "; ".join(problems) + "."
    return PASS, "Every element of columns is a JSON object carrying id, title and kind."


@export("CORE-STATE-1")
def _state_1(s):
    carried = s.columns_carrying("id")
    if not carried:
        return NA, "No column object carries an id member, so there is no identifier to judge."
    bad = [i for i, value in carried if not is_identifier(value)]
    if bad:
        return FAIL, f"The id of {positions(bad)} is not a non-empty string."
    seen = set()
    for _, value in carried:
        if value in seen:
            return FAIL, f"More than one column carries the identifier {show(value)}."
        seen.add(value)
    return PASS, "Every column's id is a non-empty string that no other column carries."


@export("CORE-STATE-2")
def _state_2(s):
    carried = s.columns_carrying("title")
    if not carried:
        return NA, "No column object carries a title member, so there is no column title to judge."
    bad = [i for i, value in carried if not is_title(value)]
    if bad:
        return FAIL, f"The title of {positions(bad)} is not a string carrying a name."
    return PASS, "Every column's title member carries a title."


@export("CORE-STATE-11")
def _state_11(s):
    carried = s.columns_carrying("kind")
    if not carried:
        return NA, "No column object carries a kind member, so there is no kind to judge."
    bad = [i for i, value in carried if not is_kind(value)]
    if bad:
        return FAIL, (f"The kind of {positions(bad)} is neither intake, work nor done as spelled there, "
                      "nor a name carrying a layer's prefix.")
    return PASS, "Every column's kind is one the profile declares or one carrying a layer's prefix."


@export("CORE-STATE-5")
def _state_5(s):
    carried = s.columns_carrying("capacity")
    if not carried:
        return NA, "No column carries a capacity member, so no column uses this permission."
    bad = [i for i, value in carried if not is_capacity(value)]
    if bad:
        return FAIL, f"The capacity of {positions(bad)} is not a whole number greater than zero."
    return PASS, f"The capacity of {positions([i for i, _ in carried])} is a whole number greater than zero."


@export("CORE-STATE-10")
def _state_10(s):
    carried = s.columns_carrying("slug")
    if not carried:
        return NA, "No column carries a slug member, and the reader judges only the slugs an object carries."
    for n, (i, value) in enumerate(carried):
        for j, other in carried[n + 1:]:
            if json_equal(value, other):
                return FAIL, f"Columns {i + 1} and {j + 1} carry the same slug {show(value)}."
    return PASS, f"The {plural(len(carried), 'slug')} the columns carry are all different."


def _top_permission(ident, members):
    @export(ident)
    def judge(s):
        carried = [m for m in members if m in s.obj]
        if carried:
            return PASS, f"The object carries {names(carried)}, which this statement permits."
        return NA, f"The object carries no {names(members, 'or')} member, so it does not use this permission."


def _column_permission(ident, members):
    @export(ident)
    def judge(s):
        columns = s.column_objects()
        carried = [m for m in members if any(m in c for _, c in columns)]
        if carried:
            return PASS, f"At least one column carries {names(carried)}, which this statement permits."
        return NA, f"No column carries {names(members, 'or')}, so no column uses this permission."


# Permissions are reported as pass where used and not applicable otherwise
# (R-12); the members' shapes are not judged (R-11).
_column_permission("CORE-STATE-4", ("operator_owned",))
_column_permission("CORE-JSON-10", JSON10_COLUMN)
_top_permission("CORE-JSON-11", ("fields", "field_values"))
_column_permission("CORE-JSON-12", JSON12_COLUMN)
_top_permission("CORE-FIELD-1", ("fields",))
_column_permission("CORE-FIELD-10", ("require_fields",))
_top_permission("CORE-CAP-1", ("tiers",))
_top_permission("CORE-JSON-13", ("tiers",))
_column_permission("CORE-GATE-1", ("gate_items",))
_column_permission("CORE-INSTR-1", ("instructions",))


def judge_statements(judges, subject, idents):
    out = []
    for ident in idents:
        judge = judges.get(ident)
        result, detail = judge(subject) if judge else (NA, NOT_IMPLEMENTED)
        out.append({"id": ident, "result": result, "detail": detail})
    return out


def verdict(statements):
    """R-17: refuse on any failure, naming the refusal section 6.1 puts first."""
    failed = [s["id"] for s in statements if s["result"] == FAIL]
    if not failed:
        return "accept", None
    reported = {REFUSAL_BY_STATEMENT.get(ident, DEFAULT_REFUSAL) for ident in failed}
    return "refuse", next(name for name in REFUSAL_ORDER if name in reported)


def check_file(path, data, profile, idents):
    statements = judge_statements(EXPORT_JUDGES, Subject(Document(data), profile), idents)
    decision, refusal = verdict(statements)
    return {"file": path, "verdict": decision, "refusal": refusal, "statements": statements}


# ---------------------------------------------------------------------------
# Judgements of a sent object and the form a tool wrote back (scope "pair")


SENT_UNREADABLE = "The sent file is not one JSON object, so there is nothing a tool could have kept."
RETURNED_UNREADABLE = "The returned file is not one JSON object, which CORE-JSON-1 reports."


class Exchange:
    def __init__(self, sent: Document, returned: Document):
        self.sent = sent
        self.returned = returned
        self.s = sent.object
        self.r = returned.object

    def returned_columns(self):
        value = self.r.get("columns") if self.r is not None else None
        return value if isinstance(value, list) else None

    def matched_columns(self):
        """R-21: each sent column object with the returned column of the same id."""
        sent = self.s.get("columns") if self.s is not None else None
        if not isinstance(sent, list):
            return []
        returned = self.returned_columns() or []
        by_id = {}
        for column in returned:
            if isinstance(column, dict) and isinstance(column.get("id"), str):
                by_id.setdefault(column["id"], column)
        pairs = []
        for i, column in enumerate(sent):
            if not isinstance(column, dict):
                continue
            if isinstance(column.get("id"), str):
                other = by_id.get(column["id"])
            else:
                other = returned[i] if i < len(returned) and isinstance(returned[i], dict) else None
            pairs.append((i, column, other))
        return pairs


PAIR_JUDGES = {}


def pair(ident):
    def register(judge):
        PAIR_JUDGES[ident] = judge
        return judge
    return register


@pair("CORE-TEXT-1")
def _text_1(x):
    if x.returned.utf8:
        return PASS, "The returned file's bytes decode as UTF-8."
    return FAIL, f"The returned file's bytes are not valid UTF-8: {x.returned.error}."


def respelled(sent, returned, defined):
    """R-22: member names the profile defines that are gone from the returned
    object while a name the sent object never carried has appeared."""
    missing = [m for m in defined if m in sent and m not in returned]
    arrived = [m for m in returned if m not in defined and m not in sent]
    return missing if arrived else []


@pair("CORE-TEXT-3")
def _text_3(x):
    if x.s is None:
        return NA, SENT_UNREADABLE
    if x.r is None:
        return NA, RETURNED_UNREADABLE
    problems = [f"the member {m}" for m in respelled(x.s, x.r, TOP_MEMBERS)]
    for i, sent, returned in x.matched_columns():
        if returned is None:
            continue     # CORE-JSON-1 reports a lost column.
        problems += [f"the member {m} of column {i + 1}"
                     for m in respelled(sent, returned, COLUMN_MEMBERS)]
        if isinstance(sent.get("kind"), str) and "kind" in returned \
                and not json_equal(sent["kind"], returned["kind"]):
            problems.append(f"the kind {show(sent['kind'])} of column {i + 1}")
    if problems:
        return FAIL, f"The returned form no longer spells {names(problems[:3])} as the profile's token."
    return PASS, "Every member name and kind the sent object carried comes back spelled as sent."


@pair("CORE-JSON-1")
def _json_1(x):
    if x.s is None or not isinstance(x.s.get("columns"), list):
        return NA, "The sent file is not an interchange object with a columns array, so there is nothing to read back."
    if x.r is None:
        if not x.returned.utf8 or not x.returned.parsed:
            return FAIL, f"The returned file is not a JSON text in UTF-8: {x.returned.error}."
        return FAIL, f"The returned JSON text is {json_kind(x.returned.value)} rather than one JSON object."
    problems = [f"the member {m}" for m in ("title",) + OPTIONAL_TOP
                if m in x.s and not (m in x.r and json_equal(x.s[m], x.r[m]))]
    sent_columns, returned_columns = x.s["columns"], x.returned_columns()
    if returned_columns is None:
        problems.append("the columns array")
    elif len(returned_columns) != len(sent_columns):
        problems.append(f"the number of columns, {len(sent_columns)} sent and {len(returned_columns)} returned")
    else:
        for i, (sent, returned) in enumerate(zip(sent_columns, returned_columns)):
            if isinstance(sent, dict) and isinstance(returned, dict):
                changed = [m for m in COLUMN_MEMBERS
                           if m in sent and not (m in returned and json_equal(sent[m], returned[m]))]
                if changed:
                    problems.append(f"{names(changed)} of column {i + 1}")
            elif not json_equal(sent, returned):
                problems.append(f"column {i + 1}")
    if problems:
        return FAIL, f"The returned form differs from the sent definition in {names(problems[:3])}."
    return PASS, "The returned form carries the sent title, columns and order, and every member the profile defines unchanged."


@pair("CORE-JSON-7")
def _json_7(x):
    if x.s is None:
        return NA, SENT_UNREADABLE
    top = [m for m in x.s if m not in TOP_MEMBERS and "." not in m]
    columns = [(i, sent, returned, [m for m in sent if m not in COLUMN_MEMBERS])
               for i, sent, returned in x.matched_columns()]
    columns = [entry for entry in columns if entry[3]]
    total = len(top) + sum(len(entry[3]) for entry in columns)
    if total == 0:
        return NA, "The sent object carries no member the profile does not define, apart from any layer CORE-LAYER-2 judges."
    if x.r is None:
        return FAIL, f"The returned file is not one JSON object, so none of the {plural(total, 'unrecognized member')} survived."
    lost = [m for m in top if not (m in x.r and json_equal(x.s[m], x.r[m]))]
    for i, sent, returned, members in columns:
        lost += [f"{m} of column {i + 1}" for m in members
                 if returned is None or m not in returned or not json_equal(sent[m], returned[m])]
    if lost:
        return FAIL, f"The returned form does not carry {names(lost[:3])} unchanged."
    return PASS, f"All {plural(total, 'unrecognized member')} of the sent object came back unchanged."


@pair("CORE-LAYER-2")
def _layer_2(x):
    if x.s is None:
        return NA, SENT_UNREADABLE
    layers = [m for m in x.s if "." in m]
    if not layers:
        return NA, "The sent object declares no layer, which the reader recognizes as a top-level name containing a full stop."
    if x.r is None:
        return FAIL, f"The returned file is not one JSON object, so the {plural(len(layers), 'layer')} declared did not survive."
    lost = [m for m in layers if not (m in x.r and json_equal(x.s[m], x.r[m]))]
    if lost:
        return FAIL, f"The returned form does not carry the layer {names(lost[:3])} unchanged."
    return PASS, f"The {plural(len(layers), 'layer')} the sent object declared came back unchanged."


# ---------------------------------------------------------------------------
# Commands


def run_check(profile, scopes, paths, blobs):
    idents = [i for i in profile.identifiers if scopes.get(i) == "export"]
    results = [check_file(path, data, profile, idents) for path, data in zip(paths, blobs)]
    return {"reader": profile.reader_member(), "results": results}, EXIT_OK


def run_pair(profile, scopes, paths, blobs):
    idents = [i for i in profile.identifiers if scopes.get(i) == "pair"]
    exchange = Exchange(Document(blobs[0]), Document(blobs[1]))
    result = {"sent": paths[0], "returned": paths[1],
              "statements": judge_statements(PAIR_JUDGES, exchange, idents)}
    return {"reader": profile.reader_member(), "result": result}, EXIT_OK


def run_scope(profile, scopes):
    published = profile.identifiers
    valid = {ident: scope for ident, scope in scopes.items() if scope in SCOPES}
    counts = {scope: sum(1 for s in valid.values() if s == scope) for scope in SCOPES}
    unclassified = [i for i in published if i.startswith("CORE-") and i not in valid]
    stale = [i for i in scopes if i not in set(published)]
    document = {"reader": profile.reader_member(), "classified": counts,
                "unclassified": unclassified, "stale": stale}
    code = EXIT_UNCLASSIFIED if any(i.startswith("CORE-JSON-") for i in unclassified) else EXIT_OK
    return document, code


class _Parser(argparse.ArgumentParser):
    # Standard output carries the JSON document alone, so help goes to stderr.
    def print_help(self, file=None):
        super().print_help(file if file is not None else sys.stderr)


def build_parser():
    parser = _Parser(prog="reader.py", description="Judge objects in the interchange form of section 5.7.")
    parser.add_argument("--profile", required=True, help="path of the profile to read")
    commands = parser.add_subparsers(dest="command", required=True, metavar="command")
    check = commands.add_parser("check", help="judge each file as an interchange object")
    check.add_argument("files", nargs="+", metavar="file")
    exchange = commands.add_parser("pair", help="judge an object together with the form a tool wrote back")
    exchange.add_argument("sent")
    exchange.add_argument("returned")
    commands.add_parser("scope", help="compare scope.json with the statements the profile publishes")
    return parser


def emit(document):
    sys.stdout.buffer.write(json.dumps(document, indent=2).encode("utf-8") + b"\n")
    sys.stdout.flush()


def main(argv=None):
    if hasattr(sys, "set_int_max_str_digits"):
        sys.set_int_max_str_digits(0)
    args = build_parser().parse_args(argv)

    try:
        profile = Profile.load(args.profile)
    except ProfileError as exc:
        diagnose(str(exc))
        return EXIT_PROFILE
    try:
        scope_identity, scopes = load_scope()
    except ScopeError as exc:
        diagnose(str(exc))
        return EXIT_INTERNAL
    if scope_identity != profile.identity:
        diagnose(f"scope.json classifies {scope_identity!r} while the profile is {profile.identity!r}")

    if args.command == "scope":
        document, code = run_scope(profile, scopes)
    else:
        paths = args.files if args.command == "check" else [args.sent, args.returned]
        blobs = []
        for path in paths:
            try:
                blobs.append(Path(path).read_bytes())
            except OSError as exc:
                # R-24: a file that cannot be opened is a command line not understood.
                diagnose(f"{path} could not be read: {exc}")
                return EXIT_USAGE
        run = run_check if args.command == "check" else run_pair
        document, code = run(profile, scopes, paths, blobs)
    emit(document)
    return code


if __name__ == "__main__":
    sys.exit(main())
