#!/usr/bin/env python3
"""Derive the journal event sets from the tree and check what format.md says.

The "Journal event schema" section of docs/design/format.md states five counts
and places four event names in or out of the sets those counts name. Every one
of those claims is derived rather than remembered, so this script derives them
again from the code and the fixtures and holds the document to the answer.

It exists because the derivation was written and thrown away twice, once by the
round that wrote the paragraph and once by the round that reviewed it. A
procedure that lives only in prose is checked when somebody rebuilds the
checker, and two people rebuilding one checker from one paragraph is the hazard
this file removes.

The five sets:

  DECLARED   every Event constant internal/contract declares.
  UNWRITTEN  the names cmd/dinah/compat_test.go's unwrittenEvents exempts.
  WRITTEN    DECLARED minus UNWRITTEN, the events some command writes.
  QUERYABLE  contract.Events, the vocabulary a query over cards accepts.
  CARD       the event names a card's own journal carries, read off every
             compatibility fixture under internal/bench/testdata/compat.

Run it from anywhere: `python scripts/derive_event_counts.py`. It prints the
derivation, then reports each document claim it checked. Exit status is 0 when
the document agrees with the tree and 1 when it does not, and a disagreement
names the claim rather than the line.
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

CONTRACT = ROOT / "internal" / "contract" / "contract.go"
COMPAT_TEST = ROOT / "cmd" / "dinah" / "compat_test.go"
FIXTURES = ROOT / "internal" / "bench" / "testdata" / "compat"
DOCUMENT = ROOT / "docs" / "design" / "format.md"

# The section whose claims this script checks. Nothing outside it is read.
SECTION_HEADING = "### Journal event schema"

WORDS = {
    1: "one", 2: "two", 3: "three", 4: "four", 5: "five", 6: "six",
    7: "seven", 8: "eight", 9: "nine", 10: "ten", 11: "eleven",
    12: "twelve", 13: "thirteen", 14: "fourteen", 15: "fifteen",
    16: "sixteen", 17: "seventeen", 18: "eighteen", 19: "nineteen",
    20: "twenty", 21: "twenty-one", 22: "twenty-two",
}

# One event-name constant declaration, capturing the name a journal carries.
EVENT_CONSTANT = re.compile(r"(?m)^\tEvent[A-Za-z]+\s+= \"([a-z_]+)\"")
# One entry of the Events slice, by constant name.
EVENTS_SLICE = re.compile(r"(?s)var Events = \[\]string\{(.*?)\n\}")
# One key of the unwrittenEvents map, by constant name.
UNWRITTEN_MAP = re.compile(r"(?s)var unwrittenEvents = map\[string\]string\{(.*?)\n\}")
CONSTANT_REFERENCE = re.compile(r"(?:contract\.)?\b(Event[A-Za-z]+)")
# One journal line's event name, in either spacing json.Marshal may produce.
JOURNAL_EVENT = re.compile(r"\"event\"\s*:\s*\"([a-z_]+)\"")
# A bare card identifier or checklist label, which published text may not carry.
BOARD_REFERENCE = re.compile(r"dinah-[0-9]+|AC-[0-9]+|OQ-[0-9]+|\bD-[0-9]+")


def word(count):
    """Return the number word the document spells a count with."""
    if count not in WORDS:
        raise SystemExit(f"no number word for {count}; extend WORDS")
    return WORDS[count]


def constant_names(source, block):
    """Return the event names a block of contract.Event references resolves to."""
    names = []
    for constant in CONSTANT_REFERENCE.findall(block):
        declaration = re.search(
            r"(?m)^\t" + constant + r"\s+= \"([a-z_]+)\"", source
        )
        if declaration is None:
            raise SystemExit(f"{constant} is referenced and internal/contract declares no such constant")
        names.append(declaration.group(1))
    return names


def derive():
    """Derive the five sets from the code and the fixtures."""
    contract_source = CONTRACT.read_text(encoding="utf-8")
    declared = set(EVENT_CONSTANT.findall(contract_source))
    if not declared:
        raise SystemExit("internal/contract declares no event constant this script can read, so its pattern has gone stale")

    slice_body = EVENTS_SLICE.search(contract_source)
    if slice_body is None:
        raise SystemExit("internal/contract declares no Events slice this script can read")
    queryable = set(constant_names(contract_source, slice_body.group(1)))

    compat_source = COMPAT_TEST.read_text(encoding="utf-8")
    map_body = UNWRITTEN_MAP.search(compat_source)
    if map_body is None:
        raise SystemExit("cmd/dinah/compat_test.go declares no unwrittenEvents map this script can read")
    unwritten = set(constant_names(contract_source, map_body.group(1)))

    written = declared - unwritten
    overlap = written & queryable

    card = set()
    journals = 0
    for path in FIXTURES.rglob("journal.ndjson"):
        if path.parent.parent.name != "cards":
            continue
        journals += 1
        card.update(JOURNAL_EVENT.findall(path.read_text(encoding="utf-8")))
    if journals == 0:
        raise SystemExit(f"no card journal found under {FIXTURES}, so the card set would be empty by accident")

    return {
        "DECLARED": declared,
        "UNWRITTEN": unwritten,
        "WRITTEN": written,
        "QUERYABLE": queryable,
        "OVERLAP": overlap,
        "CARD": card,
    }


def section(text):
    """Return the Journal event schema section, whitespace normalised."""
    start = text.find(SECTION_HEADING)
    if start < 0:
        raise SystemExit(f"{DOCUMENT} carries no {SECTION_HEADING} heading")
    end = text.find("\n### ", start + len(SECTION_HEADING))
    body = text[start:] if end < 0 else text[start:end]
    return " ".join(body.split())


def claims(sets):
    """Return the claims the document has to carry, each as label and pattern."""
    written = word(len(sets["WRITTEN"]))
    queryable = word(len(sets["QUERYABLE"]))
    checks = [
        ("declared count", rf"closed set of {word(len(sets['DECLARED']))}"),
        ("written count", rf"{written.capitalize()} of them are written by some command in this build"),
        ("queryable count", rf"A second count of {queryable} sits nearby"),
        ("overlap count", rf"{word(len(sets['OVERLAP'])).capitalize()} names sit in both counts"),
        ("card-journal count", rf"{word(len(sets['CARD'])).capitalize()} of those land on a card's own journal"),
    ]

    # The three memberships the paragraph states, each phrased from the sets
    # rather than from the sentence that is already there.
    for event in sorted(sets["QUERYABLE"] - sets["WRITTEN"]):
        checks.append((
            f"{event} placed in {queryable} and out of {written}",
            rf"`{event}`[^.]*sits in the {queryable} and outside the {written}",
        ))
    held_out = sorted(sets["WRITTEN"] - sets["QUERYABLE"])
    if held_out:
        names = "`.*`".join(re.escape(name) for name in held_out)
        checks.append((
            f"{', '.join(held_out)} placed in {written} and out of {queryable}",
            rf"`{names}`[^.]*sit in the {written} and outside the {queryable}",
        ))

    # The exceptions: overlap names that no card journal carries. The document
    # has to name each one as an exception rather than leave it inside the
    # claim about what lands on a card.
    for event in sorted(sets["OVERLAP"] - sets["CARD"]):
        checks.append((
            f"{event} named as the card-journal exception",
            rf"`{event}` is the exception",
        ))
    return checks


def main():
    sets = derive()
    for name in ("DECLARED", "UNWRITTEN", "WRITTEN", "QUERYABLE", "OVERLAP", "CARD"):
        members = sorted(sets[name])
        print(f"{name:<10} {len(members):>2}  {' '.join(members)}")
    print()

    text = DOCUMENT.read_text(encoding="utf-8")
    body = section(text)
    failures = []
    for label, pattern in claims(sets):
        if re.search(pattern, body) is None:
            failures.append(f"{label}: the section carries no statement matching /{pattern}/")
        else:
            print(f"ok    {label}")

    stray = BOARD_REFERENCE.findall(text)
    if stray:
        failures.append(f"a bare card identifier or criterion label survives in published text: {', '.join(sorted(set(stray)))}")
    else:
        print("ok    no bare card identifier in published text")

    if failures:
        print()
        for failure in failures:
            print(f"FAIL  {failure}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
