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
"""

import argparse
import json
import os
import pathlib
import subprocess
import sys

# Each measurement names the label, the argv the canonical run takes and the
# argv the compact run takes. A read takes the same argv twice, because it
# changes nothing. A claim cannot: the second run of one would meet a card the
# first run took, so the two runs claim two cards whose titles differ by one
# digit and whose answers are otherwise the same shape.
MEASUREMENTS = (
    (
        "a claim response carrying instructions and two legal moves",
        ["claim", "fx-1"],
        ["claim", "fx-2"],
    ),
    (
        "an ls listing of six ordinary cards",
        ["ls", "doing"],
        ["ls", "doing"],
    ),
)

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
        for label, canonicalArgv, compactArgv in MEASUREMENTS:
            canonical = run(args.dinah, where, env, ["--json"] + canonicalArgv)
            compact = run(args.dinah, where, env, ["--format", "compact"] + compactArgv)
            if not canonical or not compact:
                sys.exit("%s produced no output; the workbench is not in the state this expects" % label)
            if '"outcome": "refused"' in canonical or "|refused|" in compact:
                sys.exit("%s was refused, so the measurement is of a refusal rather than the answer" % label)
            before = len(encoding.encode(canonical))
            after = len(encoding.encode(compact))
            saved = 100.0 * (before - after) / before
            print("  %s:\n    canonical %d tokens, compact %d tokens, %.1f%% fewer" % (label, before, after, saved))


if __name__ == "__main__":
    main()
