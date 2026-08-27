"""Measure how many tokens the compact output form saves over canonical JSON.

Dinah writes two machine forms of the same answer, and dinah-31 claims the
compact one costs a driver loop fewer tokens. Bytes are the wrong proxy for
that claim, because a byte-pair-encoding tokenizer spends one token on each of
JSON's structural characters, so a byte saving from removing them overstates
the token saving. This script replaces the proxy with a count from a named
tokenizer.

Run it against a built binary and a throwaway workbench:

    python scripts/measure_compact_tokens.py --dinah ./dinah --root /tmp/probe

It needs tiktoken, which the module does not depend on and the binary does not
ship. Install it into a virtual environment of your own; nothing in the build
or the test suite reads this file.

Two runs against one binary report the same figures. The workbench is rebuilt
on every run, so the identifiers, revisions and claim stamps in its answers are
fresh each time, and a byte-pair encoder segments random hex unpredictably;
before anything is counted every such span is replaced with a fixed stand-in of
the same shape. Each measurement also prints a digest over the two pinned
payloads, so a reader re-running the script can see whether the figure quoted
elsewhere was taken from these bytes or from different ones.
"""

import argparse
import hashlib
import json
import os
import pathlib
import re
import subprocess
import sys

# Each measurement names the label, the argv both runs take, and the argv that
# puts the workbench back the way it was between them. A read needs nothing put
# back, because it changes nothing. A claim does: the second run of one would
# meet a card the first run took, so a release stands between the two runs and
# both of them claim the same card.
#
# Both runs measuring one card is what makes the two sides comparable. When
# they claimed two different cards, each side carried its own random
# identifiers and its own revision, which a byte-pair encoder segments
# differently, so the difference between the two counts carried noise
# belonging to neither encoding.
MEASUREMENTS = (
    (
        "a claim response carrying instructions and two legal moves",
        ["claim", "fx-1"],
        ["release", "fx-1"],
    ),
    (
        "an ls listing of six ordinary cards",
        ["ls", "doing"],
        [],
    ),
)

# The spans a fresh workbench re-rolls on every run, and what each is pinned to
# before anything is counted. A card identifier is fresh random hex, a revision
# is a sha256 over content carrying those identifiers, and a claim stamps the
# wall clock. A byte-pair encoder segments random hex unpredictably, so a claim
# answer of a few hundred tokens moved by several points between runs of one
# unchanged binary, and a quoted figure could not be checked by re-running the
# script.
#
# Each stand-in is the same shape and the same length as what it replaces, and
# one pass pins both forms, so what is compared is two encodings of one answer
# rather than two samples of noise.
# The identifier and the revision below are values a real run produced, and
# they were chosen because this tokenizer spends the median cost of their own
# distribution on them: 7 tokens for an identifier against a median of 7 over
# 2000 random ones, and 39 for a revision against a median of 39. An invented
# stand-in is the trap here. The first spelling of this pin repeated a short
# pattern to fill the sixty-four hex digits, which cost 55 tokens rather than
# 39, and those extra tokens landed on both forms and dragged the reported
# saving down by several points. Pinning removes the variance and is not meant
# to move the level, so a stand-in is measured against the distribution it
# stands for before it goes in.
PINS = (
    (re.compile(r"sha256:[0-9a-f]{64}"), "sha256:451e40cab90727cb4a128e2326db5b720e294faa3cddf69b4341e4e0cdd39203"),
    (re.compile(r"\b[0-9a-f]{12}\b"), "fa68cbea8361"),
    (re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z"), "2026-01-02T03:04:05Z"),
)


def pin(text):
    """Replace every span a fresh workbench re-rolls with a fixed stand-in of
    the same shape, so two runs of this script against one binary count the
    same bytes and report the same figure."""
    for pattern, fixed in PINS:
        text = pattern.sub(fixed, text)
    return text


def digest(text):
    """Return the leading bytes of a sha256 over a pinned payload. Two runs
    reporting one digest counted one string, so a reader who re-runs the script
    can tell a figure that reproduces from a figure that merely resembles the
    one quoted."""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:12]

# The instructions a claim serves are the same prose under either encoding, so
# they are the part of the answer neither form can make cheaper. The two
# workbenches below bracket what a real one costs: one serving none, which is
# the encoding's saving on its own, and one serving a short paragraph, which is
# what a driver loop actually reads. Report both, because a single number taken
# from either alone reads as a claim about the other.
INSTRUCTIONS = (
    "Take the card up, do the work the card describes, and move it on when the "
    "build is green. Leave a note saying what you changed and what you left "
    "undone, so whoever reads it next does not have to work that out from the "
    "diff.\n"
)


def run(dinah, root, env, argv):
    """Run one invocation of Dinah in the probe workbench and return stdout."""
    finished = subprocess.run(
        [dinah] + argv, cwd=root, env=env, capture_output=True, text=True, encoding="utf-8"
    )
    return finished.stdout


def build(dinah, root, env, instructions):
    """Populate the probe workbench: three columns, and six cards standing in
    the second of them, which is where one of them is then claimed."""
    definition = pathlib.Path(root, "definition.json")
    definition.write_text(
        json.dumps({
            "profile": "dinah-core/0.7",
            "title": "Measurement",
            "columns": [
                {"id": "d00000000001", "title": "Intake", "kind": "intake"},
                {"id": "d00000000002", "title": "Doing", "kind": "work", "instructions": instructions},
                {"id": "d00000000003", "title": "Done", "kind": "done"},
            ],
        }, indent=2),
        encoding="utf-8",
    )
    run(dinah, root, env, ["init", "--from", str(definition), "--slug", "fx", "--operator", "alka"])
    for n in range(1, 7):
        run(dinah, root, env, ["--json", "add", "Card number %d" % n])
        run(dinah, root, env, ["--json", "move", "fx-%d" % n, "doing"])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dinah", required=True, help="the built binary to measure")
    parser.add_argument("--root", required=True, help="an empty directory to build a workbench in")
    parser.add_argument("--encoding", default="cl100k_base", help="the tiktoken encoding to count with")
    args = parser.parse_args()

    import tiktoken

    encoding = tiktoken.get_encoding(args.encoding)
    print("tokenizer: tiktoken %s" % args.encoding)
    for served, description in (("", "serving no instructions"), (INSTRUCTIONS, "serving a short paragraph")):
        where = os.path.join(args.root, "with" if served else "without")
        os.makedirs(where, exist_ok=True)
        env = dict(os.environ)
        env.update({
            "DINAH_HOME": os.path.join(where, "home"),
            "DINAH_ACTOR": "alka",
            "DINAH_FORMAT": "",
            "DINAH_WORKBENCH": "",
            "DINAH_LANG": "",
        })
        build(args.dinah, where, env, served)
        print("\na workbench %s:" % description)
        for label, argv, between in MEASUREMENTS:
            canonical = run(args.dinah, where, env, ["--json"] + argv)
            if between:
                run(args.dinah, where, env, ["--json"] + between)
            compact = run(args.dinah, where, env, ["--format", "compact"] + argv)
            if not canonical or not compact:
                sys.exit("%s produced no output; the workbench is not in the state this expects" % label)
            if '"outcome": "refused"' in canonical or "|refused|" in compact:
                sys.exit("%s was refused, so the measurement is of a refusal rather than the answer" % label)
            canonical, compact = pin(canonical), pin(compact)
            before = len(encoding.encode(canonical))
            after = len(encoding.encode(compact))
            saved = 100.0 * (before - after) / before
            print("  %s:\n    canonical %d tokens, compact %d tokens, %.1f%% fewer" % (label, before, after, saved))
            print("    pinned digests: canonical %s, compact %s" % (digest(canonical), digest(compact)))


if __name__ == "__main__":
    main()
