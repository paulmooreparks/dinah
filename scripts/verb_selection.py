"""Compare verb selection under two tool-definition blocks.

A token count says how much a published tool surface costs and says nothing
about whether an agent can still find the right verb in it. This script is the
check that can fail on the second question. It takes two `dinah` binaries, the
one a cut is measured against and the one under test, starts each binary's MCP
head against a throwaway workbench, reads `tools/list` from each, and walks a
committed fixture of task statements. Each statement is one thing an agent has
decided to do, paired with the tool the surface intends that statement to
reach.

Run it against two built binaries and a throwaway root:

    python scripts/verb_selection.py --baseline ./dinah-baseline \\
        --candidate ./dinah-landing --root /tmp/verbsel \\
        --api-key-file ~/.dinah-token-count.txt

Pinning the temperature does not make an answer deterministic, and nothing here
assumes that it does. The Messages API documents temperature as a sampling
control and publishes no reproducibility guarantee at any value, so a check
reading one sample as a verdict would rest on behaviour nobody has promised.
This script samples instead: --trials requests per scenario per block, reduced
to a settled selection or to no selection at all, with the settle bar written
as a share of the trials so that one rule governs every trial count the script
accepts.

The credential is read from the file named by --api-key-file at the moment of
use and is never printed, logged, or written anywhere. Do not export it into a
shell-wide environment: a coding agent that finds ANTHROPIC_API_KEY in its own
environment may bill its work to that key by usage, which is the cost this
whole workstream exists to reduce.

No dinah command is ever run against this repository's own workbench. Every
invocation builds a fresh workbench under --root and points DINAH_HOME at a
directory inside it.
"""

import argparse
import collections
import hashlib
import json
import os
import pathlib
import shutil
import sys
import time
import urllib.error
import urllib.request

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

import measure_agentic_sequence as harness  # noqa: E402


# --------------------------------------------------------------------------
# The settle rule.
# --------------------------------------------------------------------------

# A block's selection for a scenario is settled when one tool takes at least
# this share of that scenario's trials, with the bar rounded up to a whole
# trial. Four fifths is four of five at the default trial count and twelve of
# fifteen under the escalation, so one rule governs both.
#
# Writing the bar as a share rather than as a count is what keeps the
# escalation worth running. Carried over as four raw votes, fifteen trials
# would let each of two evenly matched candidates clear the bar about
# ninety-eight times in a hundred, both would clear it in the same run, and the
# branch that decides the hard cases would be the weakest rule in the check. At
# twelve of fifteen an even toss clears the bar about eighteen times in a
# thousand, and because the bar is above a majority no two tools can clear it
# at once at any trial count.
SETTLE_NUMERATOR = 4
SETTLE_DENOMINATOR = 5

# The trial count both failing verdicts are re-run at, inside the same
# invocation. An invocation already asking for more than this escalates at its
# own count instead, because escalating downward would decide a hard case on
# less evidence than the pass that reached it.
ESCALATED_TRIALS = 15

# The pinned request settings. Every one of them is recorded in the header, and
# the two sides are compared before a verdict is printed, because a comparison
# whose sides differ in any of these is not a comparison of tool blocks.
ENDPOINT = "https://api.anthropic.com/v1/messages"
API_VERSION = "2023-06-01"
TEMPERATURE = 0.0

# The model this check chooses under by default, which is not the model the
# cost harness counts for. The sampling control has to be pinned for the header
# to mean anything, and the endpoint answers 400 with "`temperature` is
# deprecated for this model" for the newest Opus, so a run pinned against that
# model cannot be made. This default is a model the endpoint accepts the
# parameter on. Naming a model that rejects it fails the run outright, with the
# endpoint's own message, rather than quietly dropping the pin.
DEFAULT_MODEL = "claude-sonnet-4-5"

# The smallest budget that admits a single tool call. A tool_use block carries
# its own input object, and the largest schema on this surface takes a handful
# of short string fields, so the budget is set here rather than left to a value
# large enough to hide a model that answered in prose.
MAX_TOKENS = 256

FIXTURE = str(pathlib.Path(__file__).resolve().parent / "verb_selection_fixture.json")


def settle_bar(trials):
    """The whole number of trials one tool must take for a selection to settle."""
    return -(-SETTLE_NUMERATOR * trials // SETTLE_DENOMINATOR)


def settled(picks, trials):
    """The tool a block settled on for one scenario, or None when none did."""
    bar = settle_bar(trials)
    for name, count in collections.Counter(picks).items():
        if count >= bar:
            return name
    return None


# --------------------------------------------------------------------------
# The Messages API, spoken to once per trial.
# --------------------------------------------------------------------------


class Chooser(object):
    """One pinned Messages API caller, counting its own requests and tokens."""

    def __init__(self, model, key_path, system):
        self.model = model
        self.key_path = key_path
        self.system = system
        self.requests = 0
        self.input_tokens = 0
        self._key = None

    def _credential(self):
        if self._key is None:
            try:
                text = pathlib.Path(self.key_path).read_text(encoding="utf-8").strip()
            except OSError as err:
                raise harness.Failure(
                    "the credential file %s could not be read (%s)"
                    % (self.key_path, err.__class__.__name__))
            if not text:
                raise harness.Failure("the credential file %s is empty" % self.key_path)
            self._key = text
        return self._key

    def choose(self, block, statement):
        """The name of the tool the model called for one statement, or None when
        it called none. tool_choice forces a call, so None is a finding rather
        than an ordinary answer."""
        body = {
            "model": self.model,
            "max_tokens": MAX_TOKENS,
            "temperature": TEMPERATURE,
            "system": self.system,
            "tools": block,
            "tool_choice": {"type": "any"},
            "messages": [{"role": "user", "content": statement}],
        }
        encoded = json.dumps(body, ensure_ascii=False).encode("utf-8")
        request = urllib.request.Request(
            ENDPOINT,
            data=encoded,
            headers={
                "x-api-key": self._credential(),
                "anthropic-version": API_VERSION,
                "content-type": "application/json",
            },
        )
        last = None
        for attempt in range(4):
            try:
                with urllib.request.urlopen(request, timeout=120) as answer:
                    parsed = json.loads(answer.read().decode("utf-8"))
                self.requests += 1
                self.input_tokens += int(parsed.get("usage", {}).get("input_tokens", 0))
                for item in parsed.get("content", []):
                    if item.get("type") == "tool_use":
                        return item.get("name")
                return None
            except urllib.error.HTTPError as err:
                # The body says what the endpoint objected to. Nothing of the
                # credential travels in it.
                try:
                    complaint = err.read().decode("utf-8")[:400]
                except Exception:  # noqa: BLE001
                    complaint = ""
                last = "HTTP %d %s" % (err.code, complaint)
                if err.code not in (408, 429, 500, 502, 503, 529):
                    break
            except Exception as err:  # noqa: BLE001 - the reason is reported, not swallowed
                last = err.__class__.__name__
            time.sleep(1.5 * (attempt + 1))
        raise harness.Failure("%s answered %s" % (ENDPOINT, last))


# --------------------------------------------------------------------------
# The two blocks.
# --------------------------------------------------------------------------


def read_block(dinah, run_root, layers, body, attachment_name, attachment, comments):
    """The tool-definition block one binary serves, spelled the way a Messages
    request carries it. The workbench is the cost harness's own fixture, built
    from committed text, and it is torn down by the caller."""
    if os.path.isdir(run_root):
        shutil.rmtree(run_root)
    fixture = harness.Fixture(dinah, run_root, layers, body, attachment_name,
                              attachment, comments)
    fixture.build()
    session = harness.Session(dinah, fixture)
    try:
        return session.tools()
    finally:
        session.close()


def block_digest(block):
    return hashlib.sha256(
        json.dumps(block, ensure_ascii=False, sort_keys=True).encode("utf-8")
    ).hexdigest()[:16]


# --------------------------------------------------------------------------
# The comparison.
# --------------------------------------------------------------------------


Row = collections.namedtuple("Row", "expected baseline candidate verdict detail")


def draw(chooser, block, statement, trials):
    return [chooser.choose(block, statement) for _ in range(trials)]


def label(selection):
    return selection if selection else "unsettled"


def compare(chooser, blocks, scenarios, trials, escalated):
    """One row per scenario, and the rows that failed."""
    rows = []
    for scenario in scenarios:
        expected = scenario["tool"]
        statement = scenario["statement"]
        base = settled(draw(chooser, blocks["baseline"], statement, trials), trials)
        test = settled(draw(chooser, blocks["candidate"], statement, trials), trials)
        if base != expected:
            rows.append(Row(expected, label(base), label(test),
                            "no baseline",
                            "the baseline did not settle on the expected tool"))
            continue
        if test == expected:
            rows.append(Row(expected, label(base), label(test), "pass", ""))
            continue
        # Both failing verdicts take the one escalation, on both sides, inside
        # this invocation, so a verdict that reaches the exit code has already
        # been reproduced at the larger sample.
        first = "regression" if test else "lost settlement"
        base_e = settled(draw(chooser, blocks["baseline"], statement, escalated), escalated)
        test_e = settled(draw(chooser, blocks["candidate"], statement, escalated), escalated)
        if base_e != expected:
            rows.append(Row(expected, label(base_e), label(test_e),
                            "no baseline",
                            "%s at %d trials, and the baseline lost its settlement at %d"
                            % (first, trials, escalated)))
            continue
        if test_e == expected:
            rows.append(Row(expected, label(base_e), label(test_e), "pass",
                            "%s at %d trials did not survive %d"
                            % (first, trials, escalated)))
            continue
        rows.append(Row(expected, label(base_e), label(test_e), "fail",
                        "%s, reproduced at %d trials" % (first, escalated)))
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", required=True,
                        help="the binary whose block the cut is measured against")
    parser.add_argument("--candidate", required=True,
                        help="the binary whose block is under test")
    parser.add_argument("--root", required=True,
                        help="a directory the script builds two throwaway workbenches in")
    parser.add_argument("--fixture", default=FIXTURE,
                        help="the committed scenario fixture")
    parser.add_argument("--trials", type=int, default=5,
                        help="requests per scenario per block before any escalation")
    parser.add_argument("--model", default=DEFAULT_MODEL,
                        help="the model that chooses, pinned and recorded")
    parser.add_argument("--api-key-file",
                        default=os.path.join(os.path.expanduser("~"), ".dinah-token-count.txt"),
                        help="the file holding the console credential")
    parser.add_argument("--commit", default="HEAD",
                        help="the commit the fixture workbench's text is read at")
    args = parser.parse_args()
    try:
        sys.exit(run(args))
    except harness.Failure as failure:
        sys.stderr.write("%s\n" % failure)
        sys.exit(1)


def run(args):
    if args.trials < SETTLE_DENOMINATOR:
        raise harness.Failure(
            "--trials %d is below %d, and a settle bar of %d/%d decides nothing on fewer"
            % (args.trials, SETTLE_DENOMINATOR, SETTLE_NUMERATOR, SETTLE_DENOMINATOR))
    escalated = max(ESCALATED_TRIALS, args.trials)

    with open(args.fixture, encoding="utf-8") as handle:
        fixture = json.load(handle)
    scenarios = fixture["scenarios"]
    system = fixture["system"]

    repository = pathlib.Path(__file__).resolve().parents[1]
    repo = harness.Repository(repository, args.commit)
    layers = collections.OrderedDict()
    for name, path in harness.LAYER_SOURCES.items():
        text = repo.show(path)
        layers[name] = (harness.strip_frontmatter(text)
                        if path.endswith(".md") and text.startswith("---") else text)
    body = repo.show(harness.BODY_SOURCE)
    attachment = repo.show(harness.ATTACHMENT_SOURCE)
    attachment_name = os.path.basename(harness.ATTACHMENT_SOURCE)
    comments = harness.paragraphs(repo.show(harness.COMMENT_SOURCE), harness.SEEDED_COMMENTS)

    blocks = {}
    for label_name, binary in (("baseline", args.baseline), ("candidate", args.candidate)):
        blocks[label_name] = read_block(
            binary, str(pathlib.Path(args.root, label_name).resolve()),
            layers, body, attachment_name, attachment, comments)

    # The two sides must differ in the tool block and in nothing else. Every
    # other input is one value used twice by construction, and the equality is
    # asserted here rather than assumed, the way the cost harness refuses a
    # figure whose two sides were counted under different regimes.
    chooser = Chooser(args.model, args.api_key_file, system)
    pinned = {
        "model": args.model,
        "endpoint": ENDPOINT,
        "temperature": TEMPERATURE,
        "max_tokens": MAX_TOKENS,
        "system digest": hashlib.sha256(system.encode("utf-8")).hexdigest()[:16],
        "scenario order digest": hashlib.sha256(
            "\n".join(s["tool"] for s in scenarios).encode("utf-8")).hexdigest()[:16],
    }
    if blocks["baseline"] == blocks["candidate"]:
        raise harness.Failure(
            "the two binaries serve the same tool block, so this run would compare "
            "a block against itself")

    served = {name: sorted(entry["name"] for entry in block)
              for name, block in blocks.items()}
    missing = sorted(set(s["tool"] for s in scenarios) - set(served["candidate"]))
    if missing:
        raise harness.Failure(
            "the fixture names %s, which the block under test does not publish"
            % ", ".join(missing))

    lines = ["verb_selection: one committed fixture, two tool blocks, one pinned model", ""]
    for key, value in pinned.items():
        lines.append("  %-24s %s" % (key, value))
    lines.append("  %-24s %d" % ("trials per scenario", args.trials))
    lines.append("  %-24s %d of %d (%d/%d of the trials, rounded up)"
                 % ("settle bar", settle_bar(args.trials), args.trials,
                    SETTLE_NUMERATOR, SETTLE_DENOMINATOR))
    lines.append("  %-24s %d of %d" % ("escalated settle bar", settle_bar(escalated), escalated))
    lines.append("  %-24s %d" % ("scenarios", len(scenarios)))
    for name in ("baseline", "candidate"):
        lines.append("  %-24s %d tools, digest %s"
                     % ("%s block" % name, len(blocks[name]), block_digest(blocks[name])))
    lines.append("")

    rows = compare(chooser, blocks, scenarios, args.trials, escalated)

    lines.append("  %-18s %-18s %-18s %-16s %s"
                 % ("expected", "baseline", "under test", "verdict", "note"))
    for row in rows:
        lines.append("  %-18s %-18s %-18s %-16s %s"
                     % (row.expected, row.baseline, row.candidate, row.verdict, row.detail))
    lines.append("")

    counted = collections.Counter(row.verdict for row in rows)
    for verdict in ("pass", "no baseline", "fail"):
        lines.append("  %-24s %d" % (verdict, counted[verdict]))
    lines.append("  %-24s %d" % ("requests sent", chooser.requests))
    lines.append("  %-24s %d" % ("input tokens billed", chooser.input_tokens))
    lines.append("")
    sys.stdout.write("\n".join(lines) + "\n")

    if counted["fail"]:
        sys.stderr.write(
            "%d scenario(s) the baseline settled on and the block under test did not "
            "survived the escalation to %d trials\n" % (counted["fail"], escalated))
        return 1
    return 0


if __name__ == "__main__":
    main()
