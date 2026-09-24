#!/usr/bin/env python3
"""Audit the transcripts of a starved run for tool calls that left the root.

The author of the interchange reader runs as a separate claude process whose
file tools are confined to one directory. That confinement is the guarantee.
This script is evidence gathered beside it: it reads the stream-json
transcripts the runs wrote and reports every tool call that named a tool
outside the permitted five or a path outside the directory.

    python scripts/interchange_reader_audit.py --root <directory>
        --out <audit.json> <transcript.jsonl>...

The structure of the stream is not documented, so the script rests on nothing
about how objects nest. It parses each line as JSON, walks every value, and
takes each object whose type member is "tool_use" as one tool call. A stream
that carries no such object proves nothing, and the script fails closed on it.

Only path arguments are tested as paths: file_path and path on any tool, the
pattern of Glob, and the glob of Grep. The content of a Write, the strings of
an Edit and the regular expression of Grep are file text and search text, and
none of them is read as a path.

The script also reads the tool roster a transcript records. Each object whose
type member is "system", whose subtype is "init" and whose tools member is a
list of strings is taken as the roster of the session that wrote it, and every
name in it outside the permitted five is a finding. The rule is per run and
not per transcript: the audit is run over every transcript of a run together,
nothing documents whether a session resumed with --resume writes an init
object to its stream, so a run in which at least one transcript carried a
roster is not refused on that ground, whatever the other transcripts carried.
The roster is evidence beside the guarantee, and the guarantee for each round
is the --tools argument in the round-<n>-command.json the launcher wrote
before running it.

Exit status is 0 with no findings, 1 with findings, 2 on a usage error, and 3
when the transcripts carried no tool call at all or no transcript carried a
roster. Where a run has findings and also proves nothing, the findings win
and the status is 1, because the report carries them either way and a reader
of the status should not mistake an unproven run for a clean one.
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path

PERMITTED_TOOLS = ("Read", "Write", "Edit", "Glob", "Grep")

# Members read as paths on every tool, and members read as paths on one tool
# only. Grep's own pattern is a regular expression and is absent on purpose.
PATH_MEMBERS = ("file_path", "path")
TOOL_PATH_MEMBERS = {"Glob": ("pattern",), "Grep": ("glob",)}

DRIVE = re.compile(r"^[A-Za-z]:")


def normalise(path):
    """Spell a path with forward slashes and in lower case."""
    return path.replace("\\", "/").lower()


def segments(path):
    """Split a normalised path into its non-empty segments."""
    return [part for part in normalise(path).split("/") if part != ""]


def is_absolute(path):
    return path.startswith("/") or path.startswith("\\") or bool(DRIVE.match(path))


def contained(path, root_segments):
    """Report whether path lies inside the root, compared segment by segment."""
    parts = segments(path)
    return parts[: len(root_segments)] == root_segments


def tool_uses(value):
    """Yield every object under value whose type member is tool_use."""
    if isinstance(value, dict):
        if value.get("type") == "tool_use":
            yield value
        for member in value.values():
            yield from tool_uses(member)
    elif isinstance(value, list):
        for element in value:
            yield from tool_uses(element)


def rosters_in(value):
    """Yield every init object's tools list found under value."""
    if isinstance(value, dict):
        if (
            value.get("type") == "system"
            and value.get("subtype") == "init"
            and isinstance(value.get("tools"), list)
            and all(isinstance(name, str) for name in value["tools"])
        ):
            yield value["tools"]
        for member in value.values():
            yield from rosters_in(member)
    elif isinstance(value, list):
        for element in value:
            yield from rosters_in(element)


def path_arguments(name, arguments):
    """Yield the (member, value) pairs of a tool call that are paths."""
    if not isinstance(arguments, dict):
        return
    for member in PATH_MEMBERS + TOOL_PATH_MEMBERS.get(name, ()):
        value = arguments.get(member)
        if isinstance(value, str):
            yield member, value


def findings_for(name, arguments, root_segments):
    """Return the (value, why) findings one tool call yields."""
    found = []
    if name not in PERMITTED_TOOLS:
        found.append((name, "tool-not-permitted"))
    for _, value in path_arguments(name, arguments):
        if value.startswith("~"):
            found.append((value, "home-relative"))
        if is_absolute(value) and not contained(value, root_segments):
            found.append((value, "outside-root"))
        if ".." in segments(value):
            found.append((value, "parent-segment"))
    return found


def audit(root, transcripts):
    """Audit the transcripts against root and return the report."""
    # A root already absolute in either spelling is taken as written, so a
    # Windows root audited on Linux is not prefixed with the current directory.
    if not is_absolute(root):
        root = os.path.abspath(root)
    root_text = normalise(root).rstrip("/")
    root_segments = segments(root_text)
    report = {
        "root": root_text,
        "transcripts": [str(t) for t in transcripts],
        "tool_calls": 0,
        "by_tool": {},
        "unparsed_lines": 0,
        "rosters": {},
        "findings": [],
    }
    for transcript in transcripts:
        text = Path(transcript).read_text(encoding="utf-8", errors="replace")
        for number, line in enumerate(text.splitlines(), start=1):
            if not line.strip():
                continue
            try:
                value = json.loads(line)
            except json.JSONDecodeError:
                report["unparsed_lines"] += 1
                continue
            for roster in rosters_in(value):
                key = str(transcript)
                if key in report["rosters"]:
                    key = "%s:%d" % (transcript, number)
                report["rosters"][key] = sorted(roster)
                for name in roster:
                    if name not in PERMITTED_TOOLS:
                        report["findings"].append(
                            {
                                "transcript": str(transcript),
                                "line": number,
                                "tool": name,
                                "value": name,
                                "why": "roster-not-confined",
                            }
                        )
            for call in tool_uses(value):
                name = call.get("name")
                name = name if isinstance(name, str) else repr(name)
                report["tool_calls"] += 1
                report["by_tool"][name] = report["by_tool"].get(name, 0) + 1
                for offending, why in findings_for(
                    name, call.get("input"), root_segments
                ):
                    report["findings"].append(
                        {
                            "transcript": str(transcript),
                            "line": number,
                            "tool": name,
                            "value": offending,
                            "why": why,
                        }
                    )
    return report


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Audit starved-run transcripts for calls that left the root."
    )
    parser.add_argument("--root", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("transcripts", nargs="+")
    args = parser.parse_args(argv)
    report = audit(args.root, args.transcripts)
    Path(args.out).write_text(
        json.dumps(report, indent=2) + "\n", encoding="utf-8", newline="\n"
    )
    print(
        "%d tool calls, %d rosters, %d findings, %d unparsed lines"
        % (
            report["tool_calls"],
            len(report["rosters"]),
            len(report["findings"]),
            report["unparsed_lines"],
        )
    )
    if report["findings"]:
        return 1
    if report["tool_calls"] == 0:
        print("no tool call was read, so this audit proves nothing", file=sys.stderr)
        return 3
    if not report["rosters"]:
        print(
            "no roster was read from any transcript, so this audit proves nothing about the roster",
            file=sys.stderr,
        )
        return 3
    return 0


if __name__ == "__main__":
    sys.exit(main())
