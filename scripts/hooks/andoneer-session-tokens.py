#!/usr/bin/env python3
"""Stop / SessionEnd hook: report a session's token usage to Andoneer (andon-734).

This is the client half of the measurement story, and it exists because the
server half is worthless without a client that reaches it. Before this, every
token figure on the board came from a model reading a <usage> block about
itself and passing the captures to report_spend, which is neither auditable nor
checkable against anything.

Nothing here asks a model for a number. The hook reads the transcript Claude
Code already writes, sums the usage blocks the runtime recorded, and posts the
totals. No model is in the loop at any point, which is what earns the rows it
writes their measurement='harness_metered'.

WHAT IT READS. The hook payload arrives as JSON on stdin and carries
transcript_path and session_id (the same payload shape
record-isolation-cwd.py reads, and its test pins the key set). The transcript
is JSON Lines. Every assistant event carries a `message` object with `model`
and a `usage` block, and this hook sums exactly four fields per model:

    tokens_in  = input_tokens + cache_creation_input_tokens + cache_read_input_tokens
    tokens_out = output_tokens

Cached input is counted. It is real input the provider bills for, at a
discount, and leaving it out would report a fraction of a long session's input
volume as the whole of it. The discount belongs to the price half of a dollar
figure, which is andon-322's side of the line, not this hook's.

WHAT IT SENDS. One POST to /api/spend/session carrying one entry per model the
session used, with cumulative=true: these are session totals, and the server
deltas them against its cursor, so a hook that fires twice on one session
reports the increment rather than the total twice. session_key is the
runtime's own session id, which is what keeps two concurrent sessions on one
credential from overwriting each other's cursor.

POSTURE. Fail-soft, exactly like record-isolation-cwd.py: every path is wrapped,
nothing is written to stdout, and every exit is 0. A hook that blocks a session
to report spend has its priorities backwards. Unlike that recorder, this one
makes a network call, so it also carries a short timeout: a slow or unreachable
Andoneer costs the session a couple of seconds and nothing else.

CONFIGURATION. Two environment variables, both required; with either missing
the hook exits 0 having done nothing, which is the opt-out path.

    ANDONEER_URL     e.g. https://andoneer.example.com
    ANDONEER_TOKEN   an andn_ personal token, or an andn_oa_ connector token

Optional:

    ANDONEER_CARD_ID    binds the session's spend to one card. Omitted, the
                        server falls back to whatever card the credential
                        currently holds an active claim on, and writes an
                        unattributed row when there is neither. An
                        unattributed row is a real measurement: it counts
                        toward org totals and the coverage denominator, and
                        toward no per-card figure.
    ANDONEER_COLUMN_ID  the stage the work was performed in.
"""

import json
import os
import sys
import urllib.error
import urllib.request

# A model name the runtime uses for events it generated itself rather than
# events a model produced. Its usage block is not model consumption.
SYNTHETIC_MODEL = "<synthetic>"

# Anthropic model ids are the only ones Claude Code writes, so the provider is
# not in the transcript and is supplied here. A different runtime posting to
# the same endpoint sends its own.
PROVIDER = "anthropic"

TIMEOUT_SECONDS = 5


def totals_by_model(transcript_path):
    """Sum each model's usage across the transcript.

    Returns {model_id: {"tokens_in": int, "tokens_out": int}}. A malformed
    line is skipped rather than failing the read: a transcript being appended
    to while this runs can end mid-line, and losing the last event is a better
    outcome than losing the whole report.
    """
    totals = {}
    with open(transcript_path, "r", encoding="utf-8", errors="replace") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                event = json.loads(line)
            except ValueError:
                continue
            if not isinstance(event, dict) or event.get("type") != "assistant":
                continue
            message = event.get("message")
            if not isinstance(message, dict):
                continue
            usage = message.get("usage")
            if not isinstance(usage, dict):
                continue
            model = message.get("model") or ""
            if not model or model == SYNTHETIC_MODEL:
                continue

            def field(name):
                value = usage.get(name, 0)
                return value if isinstance(value, int) else 0

            bucket = totals.setdefault(model, {"tokens_in": 0, "tokens_out": 0})
            bucket["tokens_in"] += (
                field("input_tokens")
                + field("cache_creation_input_tokens")
                + field("cache_read_input_tokens")
            )
            bucket["tokens_out"] += field("output_tokens")
    return totals


def build_payload(session_id, totals, card_id="", column_id=""):
    """Build the POST body. Returns None when there is nothing worth sending."""
    entries = []
    for model in sorted(totals):
        bucket = totals[model]
        if bucket["tokens_in"] <= 0 and bucket["tokens_out"] <= 0:
            continue
        entry = {
            "provider": PROVIDER,
            "model_id": model,
            "tokens_in": bucket["tokens_in"],
            "tokens_out": bucket["tokens_out"],
        }
        if card_id:
            entry["card_id"] = card_id
        if column_id:
            entry["column_id"] = column_id
        entries.append(entry)
    if not entries:
        return None
    return {
        "session_key": session_id or "",
        "cumulative": True,
        "entries": entries,
    }


def post(base_url, token, payload):
    """POST the payload. Returns the response body, or raises."""
    url = base_url.rstrip("/") + "/api/spend/session"
    request = urllib.request.Request(
        url,
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Authorization": "Bearer " + token,
        },
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=TIMEOUT_SECONDS) as response:
        return response.read().decode("utf-8", errors="replace")


def main():
    base_url = os.environ.get("ANDONEER_URL", "").strip()
    token = os.environ.get("ANDONEER_TOKEN", "").strip()
    if not base_url or not token:
        return 0  # opt-out path: nothing configured, nothing reported

    try:
        raw = sys.stdin.read()
    except Exception:
        return 0
    try:
        hook_payload = json.loads(raw) if raw.strip() else {}
    except ValueError:
        return 0
    if not isinstance(hook_payload, dict):
        return 0

    transcript_path = hook_payload.get("transcript_path") or ""
    if not transcript_path or not os.path.isfile(transcript_path):
        return 0

    try:
        totals = totals_by_model(transcript_path)
    except Exception:
        return 0

    payload = build_payload(
        hook_payload.get("session_id") or "",
        totals,
        os.environ.get("ANDONEER_CARD_ID", "").strip(),
        os.environ.get("ANDONEER_COLUMN_ID", "").strip(),
    )
    if payload is None:
        return 0

    try:
        post(base_url, token, payload)
    except (urllib.error.URLError, OSError, ValueError):
        # An unreachable Andoneer costs the session nothing. A later post on
        # the SAME session_key carries the cumulative total anyway, so its
        # delta covers this gap, which is the whole reason the wire protocol
        # is cumulative rather than incremental. That recovery does not reach
        # the last post of a session: session_key is the runtime session id,
        # so the next session anchors against a fresh cursor row and this
        # session's final turn-group is simply lost. Recovery is per key, not
        # across keys.
        return 0
    return 0


if __name__ == "__main__":
    sys.exit(main())
