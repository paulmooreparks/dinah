"""Measure what one agentic work sequence costs over the verbs and over the files.

Dinah's content plane can be read two ways. An agent can ask the MCP head for a
card, for the instructions of the position it is standing in, and for an
attachment's bytes; or it can read the same three things off the filesystem and
keep the MCP head for the coordination acts only, which are the pull, the
comment and the move. This script runs one fixed sequence both ways over two
cards standing in one column, counts the tokens each way, and attributes the
difference across named contributions rather than reporting a single total.

Run it against a built binary and a throwaway root:

    python scripts/measure_agentic_sequence.py --dinah ./dinah --root /tmp/probe \\
        --counter api --api-key-file ~/.dinah-token-count.txt

It needs no third-party package under --counter api or --counter live. Under
--counter proxy it needs tiktoken, which the module does not depend on and the
binary does not ship. Install it into a virtual environment of your own;
nothing in the build or the test suite reads this file.

The credential is read from the file named by --api-key-file at the moment of
use and is never printed, logged or written anywhere. Do not export it into a
shell-wide environment: a coding agent that finds ANTHROPIC_API_KEY in its own
environment may bill its work to that key by usage, which is the cost this
measurement exists to reduce.

It sits beside scripts/measure_compact_tokens.py and inherits that script's
discipline. The workbench is rebuilt on every run, so the identifiers,
revisions, claim stamps and absolute paths in its answers are fresh each time,
and a byte-pair encoder segments random hex unpredictably. Every such span is
replaced with a fixed stand-in of the same shape before anything is counted,
and each pinned payload carries a digest so a reader can tell whether a figure
quoted elsewhere was taken from these bytes. That script's recorded trap
applies here too: a stand-in has to be measured against the distribution of the
values it replaces before it goes in, because an invented one can cost a
different number of tokens and move the level rather than only removing the
variance.

The instruction layers are this repository's own committed workbench text, read
with `git show <commit>:<path>`. No dinah command is ever run against this
repository's workbench, and no run reads the operator's home, because every
invocation sets DINAH_HOME to a directory under the throwaway root.
"""

import argparse
import collections
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request

# --------------------------------------------------------------------------
# The fixture, and where its text comes from.
# --------------------------------------------------------------------------

# The committed files the fixture's instruction layers are composed from. The
# global layer is the one layer this repository does not commit under .dinah,
# because a user-global layer is a machine artefact rather than workbench data,
# so the operator's own standing instructions to agents stand in for it. Every
# one of these is read with `git show`, so the text is a fact of the tree at a
# named commit rather than a paragraph written into this file.
LAYER_SOURCES = collections.OrderedDict((
    ("global", "CLAUDE.md"),
    ("standing", ".dinah/149f228d48c3/workbench.md"),
    ("column:working", ".dinah/149f228d48c3/columns/4fda9c9ca779/column.md"),
    ("column:next", ".dinah/149f228d48c3/columns/4b38abe7ebd5/column.md"),
))

# The card body and the attachment payload are committed Markdown from this
# repository as well, so their sizes are facts of the tree rather than numbers
# somebody chose. Both cards carry the same body and the same attachment: the
# two cards differ only in identity, which is what makes the second card's
# serve a clean across-card repeat rather than a difference in content.
BODY_SOURCE = "README.md"
ATTACHMENT_SOURCE = "docs/design/surfaces.md"

# The card arrives at the working column already carrying comments and links,
# because a card that reaches an implementer on this board always does. The
# three listings a card view carries are exactly what dinah-383 proposes to
# shape, so a fixture whose listings were empty would put that card's whole
# saving outside the measurement. The comment text is committed prose from this
# repository for the same reason the instruction layers are.
COMMENT_SOURCE = "docs/design/renaming-a-word.md"
SEEDED_COMMENTS = 3
SEEDED_LINKS = ("relates", "blocks")

# What the caller writes on the comment act. It is outbound payload rather than
# requested content, so it lands in the run total and in the reconciliation's
# residual rather than in any attributed figure. It is kept to one line for
# that reason.
COMMENT_TEXT = "Read the position, did the work, and left the branch green."

# The opening turn of the constructed transcript. Both runs carry the same one.
TASK_TEXT = (
    "Work the two cards standing in the working column. For each card: pull it, "
    "read the card, read the instructions of the position, read the card's first "
    "attachment, leave a handoff comment, and move the card on."
)

# --------------------------------------------------------------------------
# Pinning.
# --------------------------------------------------------------------------

# The spans a fresh workbench re-rolls on every run, and what each is pinned to
# before anything is counted. A card identifier is fresh random hex, a store
# directory is fresh random hex twice that width, a revision is a sha256 over
# content carrying those identifiers, and a claim stamps the wall clock.
#
# Each stand-in is a value a real run produced, so it costs what the values it
# replaces cost rather than what an invented string of the same shape happens to
# cost. The absolute root is pinned as well, because it is whatever directory
# the caller named and the two runs stand in sibling directories of it, so
# leaving it unpinned would put the caller's choice of scratch directory into
# the measurement and would make the two runs differ over nothing.
PINS = (
    (re.compile(r"sha256:[0-9a-f]{64}"),
     "sha256:451e40cab90727cb4a128e2326db5b720e294faa3cddf69b4341e4e0cdd39203"),
    (re.compile(r"\b[0-9a-f]{32}\b"), "01a07306b3047c8d9d07e9ad0f60f3e0"),
    (re.compile(r"\b[0-9a-f]{12}\b"), "fa68cbea8361"),
    (re.compile(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z"), "2026-01-02T03:04:05Z"),
)

# The stand-in every run root is pinned to. Both runs stand in a directory one
# level below the root the caller named, and both are pinned to this, so the
# path text the two runs carry is identical and neither carries the caller's
# own directory layout.
ROOT_PIN = "C:\\dinah-probe\\run"


def pin(text, roots=()):
    """Replace every span a fresh workbench re-rolls, and every spelling of the
    run's own root, with a fixed stand-in of the same shape.

    A root reaches the text in three spellings: doubled backslashes where it
    has been through a JSON string, single backslashes where it has not, and
    forward slashes where a tool wrote it that way. Each is replaced with the
    stand-in spelled the same way, so the two runs carry one path text and
    neither carries the directory the caller happened to name.
    """
    for root in roots:
        if not root:
            continue
        for spelling, replacement in (
            (root.replace("\\", "\\\\"), ROOT_PIN.replace("\\", "\\\\")),
            (root, ROOT_PIN),
            (root.replace("\\", "/"), ROOT_PIN.replace("\\", "/")),
        ):
            text = text.replace(spelling, replacement)
    for pattern, fixed in PINS:
        text = pattern.sub(fixed, text)
    return text


def digest(text):
    """Return the leading bytes of a sha256 over a pinned payload, so a reader
    who re-runs the script can tell a figure that reproduces from one that
    merely resembles the figure quoted."""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()[:12]


# --------------------------------------------------------------------------
# Go-compatible marshalling.
# --------------------------------------------------------------------------
#
# internal/mcp/mcp.go marshals every tool result with json.MarshalIndent and
# nothing in the tree calls SetEscapeHTML(false), so a Markdown body reaches an
# agent on one line with every newline as \n, every quotation mark as \", and
# every <, > and & as a six-character escape. Attribution needs variants of a
# payload counted the way the head would have marshalled them, so the escaping
# and the indentation are reproduced here and every payload is round-tripped
# through them before anything is measured. A payload that does not survive the
# round trip stops the run, because a variant built by a marshaller that
# disagrees with the head's would be measuring a string the head never sends.


def go_escape(text):
    """Escape a string the way encoding/json escapes one with HTML escaping
    left on, which is the default no caller in this tree disables."""
    out = []
    for ch in text:
        code = ord(ch)
        if ch == '"':
            out.append('\\"')
        elif ch == "\\":
            out.append("\\\\")
        elif ch == "\n":
            out.append("\\n")
        elif ch == "\r":
            out.append("\\r")
        elif ch == "\t":
            out.append("\\t")
        elif ch == "<":
            out.append("\\u003c")
        elif ch == ">":
            out.append("\\u003e")
        elif ch == "&":
            out.append("\\u0026")
        elif code < 0x20 or code in (0x2028, 0x2029):
            out.append("\\u%04x" % code)
        else:
            out.append(ch)
    return "".join(out)


def go_marshal_indent(value):
    """Render a value the way json.MarshalIndent(value, "", "  ") renders it."""
    out = []
    _marshal(value, 0, out)
    return "".join(out)


def _marshal(value, indent, out):
    if isinstance(value, dict):
        if not value:
            out.append("{}")
            return
        out.append("{\n")
        items = list(value.items())
        for position, (key, member) in enumerate(items):
            out.append(" " * (indent + 2))
            out.append('"%s": ' % go_escape(key))
            _marshal(member, indent + 2, out)
            out.append(",\n" if position < len(items) - 1 else "\n")
        out.append(" " * indent + "}")
        return
    if isinstance(value, list):
        if not value:
            out.append("[]")
            return
        out.append("[\n")
        for position, member in enumerate(value):
            out.append(" " * (indent + 2))
            _marshal(member, indent + 2, out)
            out.append(",\n" if position < len(value) - 1 else "\n")
        out.append(" " * indent + "]")
        return
    if isinstance(value, str):
        out.append('"' + go_escape(value) + '"')
        return
    if value is True:
        out.append("true")
        return
    if value is False:
        out.append("false")
        return
    if value is None:
        out.append("null")
        return
    out.append(json.dumps(value))


def parse_payload(text):
    """Parse a marshalled payload keeping member order, and confirm this
    module's marshaller reproduces it byte for byte."""
    parsed = json.loads(text, object_pairs_hook=collections.OrderedDict)
    again = go_marshal_indent(parsed)
    if again != text:
        raise Failure(
            "the marshaller in this script disagrees with the one the MCP head used, "
            "so no variant it builds can be trusted"
        )
    return parsed


def empty_like(value):
    """Return the empty value of the same kind, which is what emptying a member
    means: the member stays and its structure is gone."""
    if isinstance(value, dict):
        return collections.OrderedDict()
    if isinstance(value, list):
        return []
    if isinstance(value, str):
        return ""
    return value


def member_at(payload, path):
    """Read the member a dotted path names, or raise KeyError."""
    node = payload
    for step in path.split("."):
        node = node[step]
    return node


def has_member(payload, path):
    try:
        member_at(payload, path)
    except (KeyError, TypeError):
        return False
    return True


def with_emptied(payload, paths):
    """Return a copy of a payload with each named member emptied in place."""
    copy = json.loads(json.dumps(payload), object_pairs_hook=collections.OrderedDict)
    for path in paths:
        steps = path.split(".")
        node = copy
        for step in steps[:-1]:
            node = node[step]
        node[steps[-1]] = empty_like(node[steps[-1]])
    return copy


# --------------------------------------------------------------------------
# The counter, which is one component behind one boundary.
# --------------------------------------------------------------------------


class Failure(Exception):
    """Something that stops the run. No figure is printed after one."""


class CounterUnreachable(Failure):
    """The selected counter could not be reached, which AC-3 answers."""


Regime = collections.namedtuple("Regime", "counter model_or_encoding")


def regime_label(regime):
    return "[counter=%s %s=%s]" % (
        regime.counter,
        "model" if regime.counter in ("api", "live") else "encoding",
        regime.model_or_encoding,
    )


class Count(collections.namedtuple("Count", "tokens regime")):
    """One token count and the regime that produced it. Nothing in this script
    prints a number without the regime beside it, and nothing combines two
    counts whose regimes differ."""

    def __add__(self, other):
        return Count(self.tokens + require_one_regime(self, other), self.regime)

    def __sub__(self, other):
        return Count(self.tokens - require_one_regime(self, other), self.regime)


def require_one_regime(left, right):
    if left.regime != right.regime:
        raise Failure(
            "refusing to combine a figure from %s with one from %s"
            % (regime_label(left.regime), regime_label(right.regime))
        )
    return right.tokens


def ratio(numerator, denominator):
    """Return a ratio, refusing outright when the two operands were produced by
    different counters. That refusal is the whole of what keeps a proxy figure
    from being read as a measurement."""
    require_one_regime(numerator, denominator)
    if denominator.tokens == 0:
        raise Failure("a ratio against a zero total is not a figure")
    return float(numerator.tokens) / float(denominator.tokens)


class Counter(object):
    """count(messages, tools) -> Count. Every token count this script produces
    passes through one implementation of this, and no other code path counts
    anything."""

    name = ""

    def regime(self):
        raise NotImplementedError

    def count(self, messages, tools=None):
        raise NotImplementedError

    def endpoint(self):
        return ""


class ApiCounter(Counter):
    """Anthropic's /v1/messages/count_tokens over a constructed transcript.

    The count is the API's own for a named model and it is deterministic, which
    is what makes per-payload attribution possible: attribution means counting
    variants of one payload and differencing them. The endpoint's own
    documentation says the figure is an estimate a real message's input-token
    count may differ from by a small amount, and that counting applies no
    prompt-caching logic, so what it reports is a request's uncached size.
    """

    name = "api"
    URL = "https://api.anthropic.com/v1/messages/count_tokens"
    VERSION = "2023-06-01"

    def __init__(self, model, key_path):
        self.model = model
        self.key_path = key_path
        self._key = None
        self._cache = {}
        self.calls = 0

    def regime(self):
        return Regime("api", self.model)

    def endpoint(self):
        return self.URL

    def _credential(self):
        if self._key is None:
            try:
                text = pathlib.Path(self.key_path).read_text(encoding="utf-8").strip()
            except OSError as err:
                raise CounterUnreachable(
                    "counter api: the credential file %s could not be read (%s)"
                    % (self.key_path, err.__class__.__name__)
                )
            if not text:
                raise CounterUnreachable(
                    "counter api: the credential file %s is empty" % self.key_path
                )
            self._key = text
        return self._key

    def count(self, messages, tools=None):
        body = {"model": self.model, "messages": messages}
        if tools:
            body["tools"] = tools
        encoded = json.dumps(body, ensure_ascii=False).encode("utf-8")
        key = hashlib.sha256(encoded).hexdigest()
        if key in self._cache:
            return Count(self._cache[key], self.regime())
        request = urllib.request.Request(
            self.URL,
            data=encoded,
            headers={
                "x-api-key": self._credential(),
                "anthropic-version": self.VERSION,
                "content-type": "application/json",
            },
        )
        last = None
        for attempt in range(4):
            try:
                with urllib.request.urlopen(request, timeout=60) as answer:
                    parsed = json.loads(answer.read().decode("utf-8"))
                tokens = int(parsed["input_tokens"])
                self._cache[key] = tokens
                self.calls += 1
                return Count(tokens, self.regime())
            except urllib.error.HTTPError as err:
                # The body says what the endpoint objected to, and a refusal
                # reported as a bare status code sends the reader looking in the
                # wrong place. Nothing of the credential travels in it.
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
        raise CounterUnreachable("counter api: %s answered %s" % (self.URL, last))

    def probe(self):
        self.count([{"role": "user", "content": "probe"}])


class ProxyCounter(Counter):
    """A local encoder, as scripts/measure_compact_tokens.py uses cl100k_base.

    Anthropic publishes no offline tokenizer for the Claude models, so a local
    encoder counts for a different model and what it produces is a proxy rather
    than a measurement. It is here because no single counter is authoritative,
    and a harness that can be re-run under another counter is how a figure gets
    checked rather than believed.
    """

    name = "proxy"

    def __init__(self, encoding_name):
        self.encoding_name = encoding_name
        try:
            import tiktoken
        except ImportError:
            raise CounterUnreachable(
                "counter proxy: tiktoken is not installed in this interpreter"
            )
        try:
            self.encoding = tiktoken.get_encoding(encoding_name)
        except Exception as err:  # noqa: BLE001
            raise CounterUnreachable(
                "counter proxy: tiktoken has no encoding %s (%s)"
                % (encoding_name, err.__class__.__name__)
            )
        self.calls = 0

    def regime(self):
        return Regime("proxy", self.encoding_name)

    def count(self, messages, tools=None):
        rendered = json.dumps(
            {"tools": tools or [], "messages": messages}, ensure_ascii=False, sort_keys=True
        )
        self.calls += 1
        return Count(len(self.encoding.encode(rendered)), self.regime())

    def probe(self):
        self.count([{"role": "user", "content": "probe"}])


class LiveCounter(Counter):
    """An agent run driven with `claude -p --output-format json`.

    It measures the real thing, and it yields the two headline totals per run
    and nothing finer, because a live agent may vary its calls between runs and
    a varying sequence cannot be differenced. Under this selection the harness
    takes the reduced path the spec contracts for a strict live ruling: the two
    headline totals per run and their ratio inside the live regime, no
    attributed figures at all, and a line printed once per run saying so.

    This implementation drives the binary and reads the usage block off its
    result object. It refuses a run whose tool-call sequence it cannot read
    back and check against the scripted one, because a live run that may have
    done something else is not corroboration of anything.
    """

    name = "live"

    def __init__(self, model, binary="claude"):
        self.model = model
        self.binary = binary

    def regime(self):
        return Regime("live", self.model)

    def endpoint(self):
        return "claude -p --output-format json"

    def count(self, messages, tools=None):
        raise CounterUnreachable(
            "counter live: this counter measures a run rather than a payload, so it "
            "produces the two headline totals and no attributed figure"
        )

    def probe(self):
        if shutil.which(self.binary) is None:
            raise CounterUnreachable(
                "counter live: no %s binary is on PATH" % self.binary
            )

    def measure_run(self, prompt, cwd, env, mcp_config, expected_tools):
        """Run one live sequence and return its input and output token totals."""
        argv = [
            self.binary, "-p", prompt,
            "--output-format", "json",
            "--model", self.model,
            "--mcp-config", mcp_config,
        ]
        finished = subprocess.run(
            argv, cwd=cwd, env=env, capture_output=True, text=True, encoding="utf-8"
        )
        if finished.returncode != 0:
            raise CounterUnreachable(
                "counter live: %s exited %d" % (self.binary, finished.returncode)
            )
        try:
            result = json.loads(finished.stdout)
        except ValueError:
            raise CounterUnreachable(
                "counter live: the result was not the JSON object this counter reads"
            )
        usage = result.get("usage")
        if not isinstance(usage, dict) or "input_tokens" not in usage:
            raise CounterUnreachable(
                "counter live: the result object carried no usage block to read"
            )
        performed = live_tool_sequence(result)
        if performed is None:
            raise CounterUnreachable(
                "counter live: the result object carried no record of which tools the "
                "run called, so the sequence could not be checked against the scripted one"
            )
        if performed != expected_tools:
            raise CounterUnreachable(
                "counter live: the run called %d tools where the scripted sequence has %d, "
                "so it is not the sequence being measured" % (len(performed), len(expected_tools))
            )
        billed = (
            int(usage.get("input_tokens", 0))
            + int(usage.get("cache_creation_input_tokens", 0))
            + int(usage.get("cache_read_input_tokens", 0))
        )
        return Count(billed, self.regime()), Count(int(usage.get("output_tokens", 0)), self.regime())


def live_tool_sequence(result):
    """Read the tool-call sequence off a live result object, or None when the
    object carries no record of one. Nothing is inferred from a shape the
    object does not carry."""
    messages = result.get("messages")
    if not isinstance(messages, list):
        return None
    performed = []
    for message in messages:
        content = message.get("content") if isinstance(message, dict) else None
        if not isinstance(content, list):
            continue
        for block in content:
            if isinstance(block, dict) and block.get("type") == "tool_use":
                performed.append(block.get("name", ""))
    return performed


def build_counter(args):
    if args.counter == "api":
        counter = ApiCounter(args.model, args.api_key_file)
    elif args.counter == "proxy":
        counter = ProxyCounter(args.encoding)
    elif args.counter == "live":
        counter = LiveCounter(args.model, args.claude)
    else:
        raise Failure("unknown counter %s" % args.counter)
    counter.probe()
    return counter


# --------------------------------------------------------------------------
# Counting one string.
# --------------------------------------------------------------------------

# Every isolated string is counted differentially against a fixed prefix, so
# the per-message overhead the counter adds cancels and what is left is the
# string's own cost. The prefix ends in a newline so a counted string starts at
# a token boundary the prefix does not straddle.
TOKEN_PROBE_PREFIX = "measurement\n"


class StringCounter(object):
    """Counts an isolated string through the selected counter and nothing else."""

    def __init__(self, counter):
        self.counter = counter
        self.base = counter.count([{"role": "user", "content": TOKEN_PROBE_PREFIX}])
        self.cache = {}

    def of(self, text):
        if text == "":
            return Count(0, self.counter.regime())
        key = hashlib.sha256(text.encode("utf-8")).hexdigest()
        if key not in self.cache:
            whole = self.counter.count(
                [{"role": "user", "content": TOKEN_PROBE_PREFIX + text}]
            )
            self.cache[key] = whole - self.base
        return self.cache[key]


# --------------------------------------------------------------------------
# The repository, read at a commit.
# --------------------------------------------------------------------------


class Repository(object):
    def __init__(self, root, commit):
        self.root = root
        resolved = subprocess.run(
            ["git", "-C", str(root), "rev-parse", commit],
            capture_output=True, text=True, encoding="utf-8",
        )
        if resolved.returncode != 0:
            raise Failure("git could not resolve the commit %s" % commit)
        self.commit = resolved.stdout.strip()

    def show(self, path):
        """Return the committed bytes of one path at this commit, naming the
        path when the tree does not carry it."""
        finished = subprocess.run(
            ["git", "-C", str(self.root), "show", "%s:%s" % (self.commit, path)],
            capture_output=True,
        )
        if finished.returncode != 0:
            raise Failure(
                "the tree at %s carries no %s, so no layer can be composed from it"
                % (self.commit[:12], path)
            )
        return finished.stdout.decode("utf-8")


def paragraphs(text, wanted):
    """Return the first paragraphs of a committed document, which is what the
    fixture's seeded comments are made of. A comment on a real card is a
    paragraph somebody wrote, so taking paragraphs from the tree keeps the
    listing's size a fact of the repository rather than a number chosen here."""
    found = []
    for block in strip_frontmatter(text).split("\n\n"):
        trimmed = block.strip()
        if len(trimmed) > 200 and not trimmed.startswith("#"):
            found.append(trimmed)
        if len(found) == wanted:
            return found
    raise Failure("%s does not carry %d paragraphs to seed comments from"
                  % (COMMENT_SOURCE, wanted))


def strip_frontmatter(text):
    """Return the body of an anchor file, which is what a serve carries. The
    frontmatter is the file's own bookkeeping and no layer is ever served with
    it."""
    if not text.startswith("---"):
        return text
    end = text.find("\n---", 3)
    if end < 0:
        return text
    rest = text[end + 4:]
    return rest[1:] if rest.startswith("\n") else rest


# --------------------------------------------------------------------------
# The fixture workbench, rebuilt for each run.
# --------------------------------------------------------------------------

COLUMN_INTAKE = "d00000000001"
COLUMN_WORKING = "d00000000002"
COLUMN_NEXT = "d00000000003"


class Fixture(object):
    """One throwaway workbench, built from one committed definition."""

    def __init__(self, dinah, run_root, layers, body, attachment_name, attachment_bytes,
                 comments):
        self.dinah = dinah
        self.comments = comments
        self.root = str(pathlib.Path(run_root).resolve())
        self.home = os.path.join(self.root, "home")
        self.env = dict(os.environ)
        self.env.update({
            "DINAH_HOME": self.home,
            "DINAH_ACTOR": "alka",
            "DINAH_FORMAT": "",
            "DINAH_WORKBENCH": "",
            "DINAH_LANG": "",
        })
        self.layers = layers
        self.body = body
        self.attachment_name = attachment_name
        self.attachment_bytes = attachment_bytes
        self.store = ""
        self.cards = []

    def cli(self, *argv):
        finished = subprocess.run(
            [self.dinah] + list(argv), cwd=self.root, env=self.env,
            capture_output=True, text=True, encoding="utf-8",
        )
        return finished.stdout

    def build(self):
        os.makedirs(self.root, exist_ok=True)
        definition = {
            "profile": "dinah-core/0.7",
            "title": "Measurement",
            "instructions": self.layers["standing"],
            "columns": [
                {"id": COLUMN_INTAKE, "title": "Intake", "kind": "intake", "slug": "intake"},
                {"id": COLUMN_WORKING, "title": "Working", "kind": "work", "slug": "working",
                 "instructions": self.layers["column:working"]},
                {"id": COLUMN_NEXT, "title": "Next", "kind": "work", "slug": "next",
                 "instructions": self.layers["column:next"]},
                {"id": "d00000000004", "title": "Done", "kind": "done", "slug": "done"},
            ],
        }
        path = os.path.join(self.root, "definition.json")
        with open(path, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(json.dumps(definition, indent=2))
        self.cli("init", "--from", path, "--slug", "fx", "--operator", "alka")
        stores = list(pathlib.Path(self.root, ".dinah").glob("*/workbench.md"))
        if len(stores) != 1:
            raise Failure("the fixture did not build one workbench under %s" % self.root)
        self.store = str(stores[0].parent)

        base = pathlib.Path(self.home, ".dinah")
        base.mkdir(parents=True, exist_ok=True)
        with open(base / "instructions.md", "w", encoding="utf-8", newline="\n") as handle:
            handle.write(self.layers["global"])

        payload_source = os.path.join(self.root, self.attachment_name)
        with open(payload_source, "wb") as handle:
            handle.write(self.attachment_bytes.encode("utf-8"))

        for number in (1, 2):
            answer = json.loads(self.cli("--json", "add", "Card number %d" % number))
            self.cards.append({"ref": "fx-%d" % number, "id": answer["card"]["id"]})

        for card in self.cards:
            self.cli("--json", "attach", card["ref"], payload_source)
            for text in self.comments:
                self.cli("--json", "comment", card["ref"], text)

        # The body and the links are written into the anchor directly, because
        # the tool publishes no non-interactive act that writes either, and a
        # card with neither is not the card an implementer meets. This runs
        # after every command that rewrites the anchor, so nothing overwrites
        # it, and the revision is a content hash computed on read rather than a
        # stored value, so the workbench reads the card as it now stands.
        for position, card in enumerate(self.cards):
            other = self.cards[(position + 1) % len(self.cards)]
            anchor = pathlib.Path(self.store, "cards", card["id"], "card.md")
            text = anchor.read_text(encoding="utf-8")
            head, _, tail = text.partition("\n---\n")
            links = "links:\n"
            for kind in SEEDED_LINKS:
                links += "  - kind: %s\n    to: %s\n" % (kind, other["id"])
            with open(anchor, "w", encoding="utf-8", newline="\n") as handle:
                handle.write(head.rstrip("\n") + "\n" + links + "---\n"
                             + tail.lstrip("\n") + self.body)

    def card_anchor(self, card):
        return os.path.join(self.store, "cards", card["id"], "card.md")

    def attachment_payload_path(self, card):
        directory = pathlib.Path(self.store, "cards", card["id"], "attachments")
        payloads = sorted(directory.glob("*/payload/*"))
        if len(payloads) != 1:
            raise Failure("card %s does not carry exactly one attachment payload" % card["ref"])
        return str(payloads[0])

    def workbench_anchor(self):
        return os.path.join(self.store, "workbench.md")

    def column_anchor(self, column_id):
        return os.path.join(self.store, "columns", column_id, "column.md")

    def global_layer_path(self):
        return os.path.join(self.home, ".dinah", "instructions.md")


# --------------------------------------------------------------------------
# The MCP session.
# --------------------------------------------------------------------------

COORDINATION_TOOLS = ("pull", "comment", "move")


class Session(object):
    """One `dinah mcp` process spoken to over line-delimited JSON-RPC."""

    def __init__(self, dinah, fixture, allowed=None):
        self.allowed = allowed
        self.next_id = 0
        self.process = subprocess.Popen(
            [dinah, "mcp"], cwd=fixture.root, env=fixture.env,
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, encoding="utf-8", bufsize=1,
        )

    def request(self, method, params=None):
        self.next_id += 1
        message = {"jsonrpc": "2.0", "id": self.next_id, "method": method}
        if params is not None:
            message["params"] = params
        self.process.stdin.write(json.dumps(message) + "\n")
        self.process.stdin.flush()
        line = self.process.stdout.readline()
        if not line:
            raise Failure("the MCP head closed its output before answering %s" % method)
        answer = json.loads(line)
        if "error" in answer:
            raise Failure("the MCP head refused %s: %s" % (method, answer["error"]))
        return answer["result"]

    def tools(self):
        """The tool-definition block as tools/list serves it, spelled the way a
        request carries it. MCP publishes the schema under inputSchema and the
        Messages API reads it under input_schema, so the one key is renamed and
        nothing else about the block is touched."""
        served = self.request("tools/list")["tools"]
        block = []
        for entry in served:
            definition = collections.OrderedDict()
            definition["name"] = entry.get("name", "")
            definition["description"] = entry.get("description", "")
            definition["input_schema"] = entry.get("inputSchema", {"type": "object"})
            block.append(definition)
        return block

    def call(self, name, arguments):
        if self.allowed is not None and name not in self.allowed:
            raise Failure(
                "the file run may reach only %s over the MCP head, and it asked for %s"
                % (", ".join(self.allowed), name)
            )
        result = self.request("tools/call", {"name": name, "arguments": arguments})
        return result["content"][0]["text"]

    def close(self):
        try:
            self.process.stdin.close()
            self.process.wait(timeout=20)
        except Exception:  # noqa: BLE001 - a probe process that will not close is not a finding
            self.process.kill()


# --------------------------------------------------------------------------
# The sequence, held once and executed by both runs.
# --------------------------------------------------------------------------

# One declarative list rather than two scripts, so the two runs cannot drift
# apart. Each entry names the act, the tool the verb run calls for it, and
# whether the file run performs it over the MCP head or off the filesystem.
SEQUENCE = (
    ("pull", "pull", "mcp"),
    ("show-card", "show", "files"),
    ("instructions", "instructions", "files"),
    ("show-attachment", "show", "files"),
    ("comment", "comment", "mcp"),
    ("move", "move", "mcp"),
)

# Which members of each act's payload carry the thing the act names, which
# members are envelope, and which member is the served chain. The chain is on
# neither side, because the chain figure counts it and counting it twice is
# what the reconciliation's residual exists to catch. `wrapper` names the
# member a read publishes its answer under, whose own members are the ones the
# split is taken over.
#
# The envelope side is declared rather than taken as the complement of the
# other two. A complement accounts for every member by construction, so it
# could never report one falling through, which is the failure the member
# listing exists to catch: a payload carrying a member none of the three lists
# names stops the run.
RESPONSE_ENVELOPE = ["outcome", "verb", "refusal", "detail", "basis", "legal_moves",
                     "loop", "affordances", "warning", "warning_detail", "context",
                     "message", "message_values"]

ATTRIBUTION = {
    "pull": {
        "wrapper": None,
        "content": ["card"],
        "chain": ["instructions.global", "instructions.standing", "instructions.column"],
        "prose_content": [],
        "envelope": RESPONSE_ENVELOPE,
    },
    "move": {
        "wrapper": None,
        "content": ["card"],
        "chain": ["instructions.global", "instructions.standing", "instructions.column"],
        "prose_content": [],
        "envelope": RESPONSE_ENVELOPE,
    },
    "show-card": {
        "wrapper": "detail",
        "content": ["detail.card", "detail.body"],
        "chain": [],
        "prose_content": ["detail.body"],
        "envelope": ["affordances", "detail.links", "detail.attachments",
                     "detail.comments", "detail.path"],
    },
    "instructions": {
        "wrapper": "served",
        "content": [],
        "chain": ["served.instructions.global", "served.instructions.standing",
                  "served.instructions.column"],
        "prose_content": [],
        "envelope": ["affordances", "served.legal_moves", "served.loop", "served.column"],
    },
    "show-attachment": {
        "wrapper": None,
        "content": ["text"],
        "chain": [],
        "prose_content": ["text"],
        "envelope": ["affordances"],
    },
    "comment": {
        "wrapper": None,
        "content": [],
        "chain": [],
        "prose_content": [],
        "envelope": RESPONSE_ENVELOPE + ["card"],
    },
}

LAYER_OF = {"global": "global", "standing": "standing", "column": "column"}


def chain_layer_kind(path):
    return path.split(".")[-1]


# --------------------------------------------------------------------------
# Running the sequence.
# --------------------------------------------------------------------------


class Round(object):
    """One tool-call round: what the agent asked for, and what came back."""

    def __init__(self, kind, tool, request_input, result_text, act=None, card=None,
                 source_path=None, source_bytes=None):
        self.kind = kind
        self.tool = tool
        self.input = request_input
        self.result = result_text
        self.act = act
        self.card = card
        self.source_path = source_path
        self.source_bytes = source_bytes


def transcript(rounds):
    """Build the message array one run produces. An agent pays the whole
    conversation's input tokens on every request, so the array is what the
    closing request carries and its prefixes are what the earlier ones did."""
    messages = [{"role": "user", "content": TASK_TEXT}]
    for position, item in enumerate(rounds):
        use_id = "toolu_%02d" % (position + 1)
        messages.append({
            "role": "assistant",
            "content": [{"type": "tool_use", "id": use_id, "name": item.tool,
                         "input": item.input}],
        })
        messages.append({
            "role": "user",
            "content": [{"type": "tool_result", "tool_use_id": use_id,
                         "content": item.result}],
        })
    return messages


def run_verb(dinah, fixture, roots):
    """Perform all six acts over the MCP head."""
    session = Session(dinah, fixture)
    rounds = []
    try:
        tools = session.tools()
        for card in fixture.cards:
            ref = card["ref"]
            for act, tool, _ in SEQUENCE:
                if act == "pull":
                    arguments = {"column": "working", "actor": "alka"}
                elif act == "show-card":
                    arguments = {"card": ref, "actor": "alka"}
                elif act == "instructions":
                    arguments = {"card": ref, "actor": "alka"}
                elif act == "show-attachment":
                    arguments = {"card": ref + "/attachments/1/payload", "actor": "alka"}
                elif act == "comment":
                    arguments = {"card": ref, "text": COMMENT_TEXT, "actor": "alka"}
                else:
                    arguments = {"card": ref, "column": "next", "actor": "alka"}
                answer = session.call(tool, arguments)
                if act == "pull":
                    took(answer, ref)
                text = pin(answer, roots)
                rounds.append(Round("mcp", tool, pinned_input(arguments, roots), text,
                                    act=act, card=ref))
    finally:
        session.close()
    return rounds, tools


def run_file(dinah, fixture, roots):
    """Perform the coordination acts over the MCP head and read the content
    plane off the filesystem."""
    session = Session(dinah, fixture, allowed=COORDINATION_TOOLS)
    rounds = []
    try:
        tools = session.tools()
        for card in fixture.cards:
            ref = card["ref"]
            answer = session.call("pull", {"column": "working", "actor": "alka"})
            took(answer, ref)
            text = pin(answer, roots)
            rounds.append(Round("mcp", "pull", pinned_input(
                {"column": "working", "actor": "alka"}, roots), text, act="pull", card=ref))

            shell = subprocess.run(
                [dinah, "path", ref], cwd=fixture.root, env=fixture.env,
                capture_output=True, text=True, encoding="utf-8",
            )
            rounds.append(Round("shell", "Bash", {"command": "dinah path %s" % ref},
                                pin(shell.stdout, roots), act="path", card=ref))

            for label, path in (
                ("card", fixture.card_anchor(card)),
                ("global layer", fixture.global_layer_path()),
                ("standing layer", fixture.workbench_anchor()),
                ("column layer", fixture.column_anchor(COLUMN_WORKING)),
                ("attachment payload", fixture.attachment_payload_path(card)),
            ):
                absolute = str(pathlib.Path(path).resolve())
                if not absolute.lower().startswith(fixture.root.lower()):
                    raise Failure(
                        "the file run tried to read %s, which is outside the throwaway root"
                        % absolute
                    )
                raw = pathlib.Path(absolute).read_bytes()
                content = raw.decode("utf-8")
                rounds.append(Round(
                    "file", "Read", pinned_input({"file_path": absolute}, roots),
                    pin(content, roots), act="read:" + label, card=ref,
                    source_path=absolute, source_bytes=len(raw),
                ))

            text = pin(session.call(
                "comment", {"card": ref, "text": COMMENT_TEXT, "actor": "alka"}), roots)
            rounds.append(Round("mcp", "comment", pinned_input(
                {"card": ref, "text": COMMENT_TEXT, "actor": "alka"}, roots),
                text, act="comment", card=ref))

            text = pin(session.call(
                "move", {"card": ref, "column": "next", "actor": "alka"}), roots)
            rounds.append(Round("mcp", "move", pinned_input(
                {"card": ref, "column": "next", "actor": "alka"}, roots),
                text, act="move", card=ref))
    finally:
        session.close()
    return rounds, tools


def took(answer, ref):
    """Confirm a pull took the card the rest of the acts are about. A pull names
    a column rather than a card, so which card it takes is the workbench's
    choice, and a run whose later acts addressed a card the pull did not take
    would be two sequences rather than one."""
    parsed = json.loads(answer)
    taken = parsed.get("card", {}).get("ref")
    if taken != ref:
        raise Failure("the pull took %s where the sequence expected %s" % (taken, ref))


def pinned_input(arguments, roots):
    """Pin the paths and identifiers a tool call's own arguments carry, so the
    outbound half of a round is as stable as the inbound half."""
    return json.loads(pin(json.dumps(arguments), roots),
                      object_pairs_hook=collections.OrderedDict)


# --------------------------------------------------------------------------
# The two totals.
# --------------------------------------------------------------------------


def totals(counter, rounds, tools):
    """Return the context footprint and the cumulative billed input for one run.

    The footprint is the input-token count of the final transcript with the
    tool definitions included, which is what "how much context does this cost"
    means and which is invariant to caching. The cumulative figure is the sum
    over the requests the sequence produces, which is one per tool-call round
    of any kind plus the closing turn, computed under no caching.
    """
    messages = transcript(rounds)
    cumulative = None
    footprint = None
    for k in range(len(rounds) + 1):
        prefix = messages[:1 + 2 * k]
        count = counter.count(prefix, tools)
        cumulative = count if cumulative is None else cumulative + count
        footprint = count
    return footprint, cumulative


# --------------------------------------------------------------------------
# Attribution.
# --------------------------------------------------------------------------


class Attribution(object):
    """The attributed figures for one run, computed act by act."""

    def __init__(self, strings, counter):
        self.strings = strings
        self.counter = counter
        self.zero = Count(0, counter.regime())
        self.envelope = self.zero
        self.requested = self.zero
        self.reencoding = self.zero
        self.chain_arrival = collections.OrderedDict(
            (kind, self.zero) for kind in ("global", "standing", "column"))
        self.chain_repeat_within = collections.OrderedDict(
            (kind, self.zero) for kind in ("global", "standing", "column"))
        self.chain_repeat_across = collections.OrderedDict(
            (kind, self.zero) for kind in ("global", "standing", "column"))
        self.repeat_counts = collections.OrderedDict(
            (kind, {"arrival": 0, "within": 0, "across": 0})
            for kind in ("global", "standing", "column"))
        self.member_report = []
        self.act_checks = []
        self._seen = {}

    def classify(self, layer_kind, text, card):
        """Say whether a served layer is an arrival or a repeat, and where the
        most recent earlier serve of that byte-identical text fell.

        A repeat serve is a layer whose text is byte-identical to one already
        served earlier in the same sequence, and the definition covers all
        three layers rather than the column-scoped one alone. The global layer
        and the standing layer are workbench-wide, so they repeat from the
        second serve onward whatever route the cards take. The column layer
        repeats inside one card because the third act re-asks for the position
        the pull already served, and it repeats across a card boundary because
        a second card is worked at the column the first was worked at.
        """
        key = (layer_kind, text)
        previous = self._seen.get(key)
        self._seen[key] = card
        if previous is None:
            return "arrival"
        return "within" if previous == card else "across"

    def act(self, item):
        if item.kind != "mcp" or item.act not in ATTRIBUTION:
            return
        rules = ATTRIBUTION[item.act]
        payload = parse_payload(item.result)
        whole = self.strings.of(item.result)

        content_paths = [p for p in rules["content"] if has_member(payload, p)]
        chain_paths = [p for p in rules["chain"] if has_member(payload, p)]
        emptied = with_emptied(payload, content_paths + chain_paths)
        envelope = self.strings.of(go_marshal_indent(emptied))
        self.envelope = self.envelope + envelope

        act_sum = envelope

        for path in chain_paths:
            text = member_at(payload, path)
            if not text:
                continue
            kind = chain_layer_kind(path)
            raw = self.strings.of(text)
            escaped = self.escaped_cost(item.result, text)
            self.reencoding = self.reencoding + (escaped - raw)
            act_sum = act_sum + escaped
            where = self.classify(kind, text, item.card)
            self.repeat_counts[kind][where] += 1
            if where == "arrival":
                self.chain_arrival[kind] = self.chain_arrival[kind] + raw
            elif where == "within":
                self.chain_repeat_within[kind] = self.chain_repeat_within[kind] + raw
            else:
                self.chain_repeat_across[kind] = self.chain_repeat_across[kind] + raw

        for path in content_paths:
            value = member_at(payload, path)
            if path in rules["prose_content"] and isinstance(value, str):
                if not value:
                    continue
                raw = self.strings.of(value)
                escaped = self.escaped_cost(item.result, value)
                self.reencoding = self.reencoding + (escaped - raw)
                self.requested = self.requested + raw
                act_sum = act_sum + escaped
            else:
                without = self.strings.of(go_marshal_indent(with_emptied(payload, [path])))
                cost = whole - without
                self.requested = self.requested + cost
                act_sum = act_sum + cost

        self.member_report.append(self.members(item, payload, rules, content_paths, chain_paths))
        self.act_checks.append((item.card, item.act, whole, act_sum))

    def escaped_cost(self, payload_text, value):
        """Count a prose member as it appears inside the marshalled payload.

        The escaped form is built here and confirmed to occur in the payload
        the head actually sent, so a disagreement between this script's
        escaping and encoding/json's stops the run instead of quietly moving
        the re-encoding figure.
        """
        escaped = go_escape(value)
        if escaped not in payload_text:
            raise Failure(
                "the escaped form of a prose member does not occur in the payload the "
                "MCP head sent, so the re-encoding figure would be measuring a string "
                "nobody was served"
            )
        return self.strings.of(escaped)

    @staticmethod
    def members(item, payload, rules, content_paths, chain_paths):
        """List which members the act's payload carried on each side of the
        line, so the split is inspectable rather than buried in this file."""
        wrapper = rules["wrapper"]
        present = []
        for key in payload.keys():
            if wrapper and key == wrapper:
                for inner in payload[key].keys():
                    present.append("%s.%s" % (key, inner))
            else:
                present.append(key)
        chain_roots = set(p.rsplit(".", 1)[0] for p in rules["chain"])
        content = [p for p in present if p in content_paths]
        chain = [p for p in present if p in chain_roots]
        envelope = [p for p in present if p in rules["envelope"]]
        named = set(content) | set(chain) | set(envelope)
        unaccounted = [p for p in present if p not in named]
        return {
            "card": item.card, "act": item.act,
            "content": content, "envelope": envelope, "chain": chain,
            "unaccounted": unaccounted,
        }

    def chain_total(self, table):
        total = self.zero
        for value in table.values():
            total = total + value
        return total


# --------------------------------------------------------------------------
# The report.
# --------------------------------------------------------------------------


class Report(object):
    """Every line the harness prints, held until the run finishes.

    Nothing reaches stdout until every figure has been produced, because a
    harness that prints some figures and then fails has published a
    measurement it did not finish making.
    """

    def __init__(self, regime):
        self.lines = []
        self.regime = regime

    def say(self, text=""):
        self.lines.append(text)

    def figure(self, label, count, suffix=""):
        self.lines.append("  %-58s %10d tokens %s%s"
                          % (label, count.tokens, regime_label(count.regime),
                             (" " + suffix) if suffix else ""))

    def signed(self, label, count, suffix=""):
        self.lines.append("  %-58s %+10d tokens %s%s"
                          % (label, count.tokens, regime_label(count.regime),
                             (" " + suffix) if suffix else ""))

    def measure(self, label, value, unit):
        self.lines.append("  %-58s %10d %s [not a token count]" % (label, value, unit))

    def share(self, label, value):
        self.lines.append("  %-58s %+10.3f %%  [derived within one regime]"
                          % (label, value))

    def plain(self, label, value):
        self.lines.append("  %-58s %s" % (label, value))

    def emit(self):
        sys.stdout.write("\n".join(self.lines) + "\n")


# --------------------------------------------------------------------------
# Main.
# --------------------------------------------------------------------------


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dinah", required=True, help="the built binary to measure")
    parser.add_argument("--root", required=True,
                        help="a directory the harness builds two throwaway workbenches in")
    parser.add_argument("--counter", default="api", choices=("api", "live", "proxy"),
                        help="which implementation of the counter interface to count with")
    parser.add_argument("--model", default="claude-opus-5",
                        help="the model the api and live counters count for")
    parser.add_argument("--encoding", default="cl100k_base",
                        help="the tiktoken encoding the proxy counter counts with")
    parser.add_argument("--api-key-file",
                        default=os.path.join(os.path.expanduser("~"), ".dinah-token-count.txt"),
                        help="the file holding the console credential the api counter reads")
    parser.add_argument("--claude", default="claude",
                        help="the binary the live counter drives")
    parser.add_argument("--commit", default="HEAD",
                        help="the commit the instruction layers are read at")
    parser.add_argument("--cards", type=int, default=2,
                        help="how many cards the sequence carries through the column")
    parser.add_argument("--residual-bound", type=float, default=2.0,
                        help="the share of the footprint the reconciliation may miss by")
    args = parser.parse_args()

    try:
        sys.exit(measure(args))
    except Failure as failure:
        sys.stderr.write("%s\n" % failure)
        sys.exit(1)


def measure(args):
    repository = pathlib.Path(__file__).resolve().parents[1]
    repo = Repository(repository, args.commit)

    layers = collections.OrderedDict()
    provenance = []
    for name, path in LAYER_SOURCES.items():
        text = repo.show(path)
        body = strip_frontmatter(text) if path.endswith(".md") and text.startswith("---") else text
        layers[name] = body
        provenance.append((name, path, len(body.encode("utf-8"))))
    body = repo.show(BODY_SOURCE)
    attachment = repo.show(ATTACHMENT_SOURCE)
    attachment_name = os.path.basename(ATTACHMENT_SOURCE)
    comments = paragraphs(repo.show(COMMENT_SOURCE), SEEDED_COMMENTS)

    counter = build_counter(args)
    regime = counter.regime()
    report = Report(regime)

    report.say("measure_agentic_sequence: one agentic work sequence, over the verbs and over the files")
    report.plain("commit", repo.commit)
    report.plain("counter", "%s, %s" % (counter.name, regime_label(regime)))
    report.plain("endpoint", counter.endpoint() or "(none)")
    report.measure("cards in the sequence", args.cards, "cards")
    report.say()

    report.say("layers, composed from committed workbench text with git show <commit>:<path>")
    for name, path, size in provenance:
        report.plain("%-16s %s" % (name, path), "%d bytes [bytes]" % size)
    report.plain("%-16s %s" % ("card body", BODY_SOURCE),
                 "%d bytes [bytes], digest %s" % (len(body.encode("utf-8")), digest(body)))
    report.plain("%-16s %s" % ("attachment", ATTACHMENT_SOURCE),
                 "%d bytes [bytes], digest %s" % (len(attachment.encode("utf-8")),
                                                  digest(attachment)))
    report.plain("%-16s %s" % ("card comments", COMMENT_SOURCE),
                 "%d paragraphs [not a token count], %d bytes [bytes], digest %s"
                 % (len(comments), len("".join(comments).encode("utf-8")),
                    digest("".join(comments))))
    report.plain("%-16s %s" % ("card links", "each card links to the other"),
                 "%d links [not a token count] of kinds %s"
                 % (len(SEEDED_LINKS), ", ".join(SEEDED_LINKS)))
    report.say()

    if counter.name == "live":
        return reduced_live_run(args, report, counter, layers, body, attachment,
                                attachment_name, comments)

    roots = []
    runs = {}
    for label in ("verb", "file"):
        run_root = str(pathlib.Path(args.root, label).resolve())
        if os.path.isdir(run_root):
            shutil.rmtree(run_root)
        fixture = Fixture(args.dinah, run_root, layers, body, attachment_name, attachment,
                          comments)
        fixture.build()
        fixture.cards = fixture.cards[:args.cards]
        roots.append(fixture.root)
        runs[label] = fixture

    # Both runs pin both roots, so the two never differ over a directory name.
    verb_rounds, tools = run_verb(args.dinah, runs["verb"], roots)
    file_rounds, file_tools = run_file(args.dinah, runs["file"], roots)

    # AC-1: the three coordination acts are performed identically by both runs,
    # so their pinned digests agree or the run is not one measurement.
    report.say("coordination acts, pinned digests, which prove the two runs are one sequence")
    disagreed = []
    verb_coordination = [r for r in verb_rounds if r.tool in COORDINATION_TOOLS]
    file_coordination = [r for r in file_rounds if r.tool in COORDINATION_TOOLS]
    if len(verb_coordination) != len(file_coordination):
        raise Failure("the two runs performed different numbers of coordination acts")
    for left, right in zip(verb_coordination, file_coordination):
        agree = digest(left.result) == digest(right.result)
        report.plain("%s %s" % (left.card, left.tool),
                     "verb %s, file %s, %s" % (digest(left.result), digest(right.result),
                                               "agree" if agree else "DISAGREE"))
        if not agree:
            disagreed.append((left.card, left.tool))
    report.say()

    strings = StringCounter(counter)

    report.say("headline totals, one line per run, with the caching assumption each rests on")
    footprints = {}
    cumulatives = {}
    for label, rounds, tool_block in (("verb", verb_rounds, tools),
                                      ("file", file_rounds, file_tools)):
        footprint, cumulative = totals(counter, rounds, tool_block)
        footprints[label] = footprint
        cumulatives[label] = cumulative
        report.figure("%s run, context footprint" % label, footprint,
                      "(the final transcript, tool definitions included; invariant to caching)")
        report.figure("%s run, cumulative billed input" % label, cumulative,
                      "(%d requests, computed under no caching; an upper bound on what a "
                      "caching session pays, not a bill)" % (len(rounds) + 1))
    report.say()
    report.share("footprint, file run as a share of the verb run",
                 100.0 * ratio(footprints["file"], footprints["verb"]))
    report.share("cumulative, file run as a share of the verb run",
                 100.0 * ratio(cumulatives["file"], cumulatives["verb"]))
    report.signed("footprint, file run less verb run",
                  footprints["file"] - footprints["verb"])
    report.signed("cumulative, file run less verb run",
                  cumulatives["file"] - cumulatives["verb"])
    report.say()

    report.say("what the file run read, each path under the throwaway root")
    for item in file_rounds:
        if item.kind == "file":
            report.plain(item.act, "%s (%d bytes [bytes])"
                         % (pin(item.source_path, roots), item.source_bytes))
    report.plain("MCP tools the file run reached",
                 ", ".join(sorted(set(r.tool for r in file_rounds if r.kind == "mcp"))))
    report.say()

    attribution = Attribution(strings, counter)
    for item in verb_rounds:
        attribution.act(item)

    report.say("requested content and envelope, member by member, per act")
    for entry in attribution.member_report:
        report.plain("%s %s" % (entry["card"], entry["act"]),
                     "content: %s | envelope: %s | chain: %s"
                     % (", ".join(entry["content"]) or "(none)",
                        ", ".join(entry["envelope"]) or "(none)",
                        ", ".join(entry["chain"]) or "(none)"))
        if entry["unaccounted"]:
            raise Failure(
                "the payload of %s on %s carried members on neither side of the line: %s"
                % (entry["act"], entry["card"], ", ".join(entry["unaccounted"])))
    report.say()

    report.say("served instruction chain, per layer, arrivals and repeats")
    for kind in ("global", "standing", "column"):
        counts = attribution.repeat_counts[kind]
        report.figure("%s layer, arrival serves (%d)" % (kind, counts["arrival"]),
                      attribution.chain_arrival[kind])
        report.figure("%s layer, repeats within one card's own acts (%d)"
                      % (kind, counts["within"]), attribution.chain_repeat_within[kind])
        report.figure("%s layer, repeats across a card boundary (%d)"
                      % (kind, counts["across"]), attribution.chain_repeat_across[kind])
    arrivals = attribution.chain_total(attribution.chain_arrival)
    repeats_within = attribution.chain_total(attribution.chain_repeat_within)
    repeats_across = attribution.chain_total(attribution.chain_repeat_across)
    repeats = repeats_within + repeats_across
    report.figure("chain, arrival serves, all layers", arrivals)
    report.figure("chain, repeat serves, all layers", repeats)
    report.say()

    report.say("the attributed figures, as counts rather than as shares")
    report.figure("arrival serves of the instruction chain", arrivals)
    report.figure("repeat serves of the instruction chain", repeats)
    report.figure("JSON re-encoding of the prose members", attribution.reencoding)
    report.figure("response envelope, measured directly", attribution.envelope)
    report.figure("requested content", attribution.requested)
    report.say()

    tool_block = counter.count([{"role": "user", "content": TOKEN_PROBE_PREFIX}], tools) \
        - counter.count([{"role": "user", "content": TOKEN_PROBE_PREFIX}])
    verb_rounds_count = len(verb_rounds)
    file_rounds_count = len(file_rounds)
    verb_product = Count(tool_block.tokens * verb_rounds_count, regime)
    file_product = Count(tool_block.tokens * file_rounds_count, regime)
    report.say("the tool-definition block, and the round trips it is paid on")
    report.measure("tools the MCP head serves", len(tools), "tools")
    report.figure("tool-definition block, once", tool_block)
    report.measure("verb run, tool-call rounds", verb_rounds_count, "rounds")
    report.measure("file run, tool-call rounds", file_rounds_count, "rounds")
    report.figure("tool block over the verb run's rounds", verb_product)
    report.figure("tool block over the file run's rounds", file_product)
    report.signed("round-trip component, file run less verb run", file_product - verb_product)
    report.say()

    report.say("the reconciliation, against the verb run's context footprint")
    attributed = (arrivals + repeats + attribution.reencoding + attribution.envelope
                  + attribution.requested + tool_block)
    report.figure("sum of the attributed figures plus the requested content", attributed)
    report.figure("verb run, context footprint", footprints["verb"])
    residual = footprints["verb"] - attributed
    report.signed("residual", residual)
    residual_share = 100.0 * float(residual.tokens) / float(footprints["verb"].tokens)
    report.share("residual as a share of the footprint", residual_share)
    report.say()

    report.say("per-act check, the payload against the figures attributed to it")
    for card, act, whole, act_sum in attribution.act_checks:
        report.signed("%s %s, payload less attributed" % (card, act), whole - act_sum)
    report.say()

    report.say("what this run does not measure")
    report.plain("listing acts", "none is in the sequence, so no figure here tracks how "
                                 "many cards a workbench holds")
    report.plain("caching", "the api counter applies no prompt-caching logic, so every "
                            "figure above is an uncached request size")
    report.plain("the file run's own tools", "Read and Bash belong to the agent harness "
                                             "rather than to this surface, so the tool "
                                             "block counted above is the MCP head's alone")
    report.say()

    failed = []
    if disagreed:
        failed.append("the two runs disagreed on %d coordination acts" % len(disagreed))
    if abs(residual_share) > args.residual_bound:
        failed.append("the reconciliation residual is %.3f%% of the footprint, outside the "
                      "%.1f%% bound, so no figure from this run may be quoted"
                      % (residual_share, args.residual_bound))
    if attribution.repeat_counts["column"]["across"] <= 0:
        failed.append("the column layer repeated across no card boundary, so this sequence "
                      "is not the one the decision to carry two cards rests on")

    report.emit()
    if failed:
        for line in failed:
            sys.stderr.write("%s\n" % line)
        return 1
    return 0


def reduced_live_run(args, report, counter, layers, body, attachment, attachment_name,
                     comments):
    """The contract for a strict live ruling: the two headline totals per run and
    their ratio inside the live regime, no attributed figures at all, and a line
    saying so ahead of the totals."""
    report.say("counter live: this run produces the two headline totals and no attribution, "
               "because a live agent may vary its calls between runs and a varying sequence "
               "cannot be differenced. AC-6, AC-7, AC-8 and AC-14 are withdrawn under this "
               "selection, and this run bounds none of dinah-380, dinah-382 or dinah-383 "
               "individually.")
    report.say()
    totals_by_run = {}
    for label in ("verb", "file"):
        run_root = str(pathlib.Path(args.root, label).resolve())
        if os.path.isdir(run_root):
            shutil.rmtree(run_root)
        fixture = Fixture(args.dinah, run_root, layers, body, attachment_name, attachment,
                          comments)
        fixture.build()
        fixture.cards = fixture.cards[:args.cards]
        config = os.path.join(fixture.root, "mcp.json")
        with open(config, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(json.dumps({"mcpServers": {"dinah": {
                "command": args.dinah, "args": ["mcp"],
                "env": {"DINAH_HOME": fixture.home}}}}, indent=2))
        expected = live_expected_tools(label, args.cards)
        billed, produced = counter.measure_run(
            live_prompt(label, fixture), fixture.root, fixture.env, config, expected)
        totals_by_run[label] = billed
        report.figure("%s run, cumulative billed input" % label, billed,
                      "(read from the usage block of a real run, so caching applied)")
        report.figure("%s run, output tokens" % label, produced)
    report.say()
    report.share("cumulative, file run as a share of the verb run",
                 100.0 * ratio(totals_by_run["file"], totals_by_run["verb"]))
    report.emit()
    return 0


def live_prompt(label, fixture):
    if label == "verb":
        return (TASK_TEXT + " Use the dinah MCP tools for every act, including the reads.")
    return (TASK_TEXT + " Use the dinah MCP tools only for the pull, the comment and the "
            "move, and read the card, the instructions and the attachment off the "
            "filesystem under " + fixture.root + " instead.")


def live_expected_tools(label, cards):
    if label == "verb":
        return ["pull", "show", "instructions", "show", "comment", "move"] * cards
    return ["pull", "Bash", "Read", "Read", "Read", "Read", "Read", "comment", "move"] * cards


if __name__ == "__main__":
    main()
