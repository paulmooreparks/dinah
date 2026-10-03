#!/usr/bin/env python3
"""Capture the storage migration goldens of dinah-637 with one dinah binary.

The goldens are the answers the build at the merge base of the dinah-637
branch gives on the fixture in before/, captured before any code of that card
changed. cmd/dinah's storage migration test replays every row of golden.json
against the migrated fixture with the new build and compares the answers byte
for byte after the one normalisation both sides apply, normalise() below.

One transformation follows the capture. The old layout's writers appended an
item's key when an act first set it, so an item.md whose evidence was set
after filing, or which was cited before it was settled, carries its keys out
of the order section 6.1 of the specification fixes. The operator ruled on
dinah-637/questions/23 that every item in the card-unit layout prints that
fixed order, its content unchanged, and that the goldens are re-captured to
it, so canonical_item_keys() below puts the keys of every item anchor an
answer carries into that order and changes nothing else. A merge that moves
the merge base re-captures with the new base's build and runs it again.

Usage: capture-goldens.py <dinah binary> <scratch directory>

The scratch directory must not exist. The fixture is copied into it inside a
.dinah container under its own workbench identifier, and every command runs
there with an isolated DINAH_HOME.
"""

import json
import os
import re
import shutil
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
FIXTURE = os.path.join(HERE, "before")
GOLDEN = os.path.join(HERE, "golden")

# WORKBENCH_ID is the identifier the fixture's store was created under. The
# store's directory name is its identifier, and before/ drops the name, so the
# test and this script both put it back from here.
WORKBENCH_ID = open(os.path.join(HERE, "workbench-id.txt")).read().strip()


def normalise(text, store):
    """Replace the store's own path with <store>, in both spellings a path
    reaches an answer in, and turn every separator after it into a slash, so
    that an answer captured on one machine compares with one given on
    another."""
    escaped = store.replace("\\", "\\\\")
    text = text.replace(escaped, "<store>").replace(store, "<store>")
    return re.sub(r"<store>[^\s\"']*",
                  lambda m: m.group(0).replace("\\\\", "/").replace("\\", "/"),
                  text)


# ITEM_KEYS is the order section 6.1 fixes for a composed item anchor. A key
# outside it, which no build writes, would keep its place after these.
ITEM_KEYS = ["kind", "state", "column", "owner", "standing", "evidence", "ts",
             "ordinal", "resolution", "citations"]

# ITEM_ANCHOR matches the frontmatter of an anchor carried as a JSON string
# member named text, whose newlines an answer spells as the two characters
# backslash and n, which is how show --json and the MCP show tool carry an
# item's anchor.
ITEM_ANCHOR = re.compile(r'"text": "---\\n(.*?)\\n---\\n')


def canonical_item_keys(text):
    """Put the keys of every item anchor in text into ITEM_KEYS order. A key's
    entry is its own line and the indented or dashed lines under it, which is
    how a block value such as citations is written, and entries move whole.
    An anchor whose first key is not kind is not an item's and is left as it
    stands."""
    def reorder(match):
        entries = []
        for line in match.group(1).split("\\n"):
            if entries and line[:1] in (" ", "-"):
                entries[-1].append(line)
            else:
                entries.append([line])
        if not entries or not entries[0][0].startswith("kind:"):
            return match.group(0)
        def rank(entry):
            key = entry[0].split(":", 1)[0]
            return ITEM_KEYS.index(key) if key in ITEM_KEYS else len(ITEM_KEYS)
        entries.sort(key=rank)
        lines = [line for entry in entries for line in entry]
        return '"text": "---\\n' + "\\n".join(lines) + '\\n---\\n'
    return ITEM_ANCHOR.sub(reorder, text)


def run(binary, store, env, args, stdin=b""):
    proc = subprocess.run([binary, "--workbench", store] + args, input=stdin,
                          capture_output=True, env=env)
    return proc.returncode, proc.stdout.decode("utf-8"), proc.stderr.decode("utf-8")


def mcp_show(binary, store, env, refs):
    """Ask the MCP server's show tool for each reference over one stdio
    session, answering the text of each tool result in order."""
    proc = subprocess.Popen([binary, "mcp", "--root", store], stdin=subprocess.PIPE,
                            stdout=subprocess.PIPE, env=env)
    def send(message):
        proc.stdin.write((json.dumps(message) + "\n").encode("utf-8"))
        proc.stdin.flush()
    def receive():
        while True:
            line = proc.stdout.readline()
            if not line:
                raise RuntimeError("mcp server closed its output")
            message = json.loads(line)
            if "id" in message:
                return message
    send({"jsonrpc": "2.0", "id": 1, "method": "initialize",
          "params": {"protocolVersion": "2025-06-18", "capabilities": {},
                     "clientInfo": {"name": "capture-goldens", "version": "1"}}})
    receive()
    send({"jsonrpc": "2.0", "method": "notifications/initialized"})
    answers = []
    for n, ref in enumerate(refs):
        send({"jsonrpc": "2.0", "id": 10 + n, "method": "tools/call",
              "params": {"name": "show", "arguments": {"card": ref, "workbench": store}}})
        reply = receive()
        answers.append("".join(part.get("text", "") for part in reply["result"]["content"]))
    proc.stdin.close()
    proc.wait()
    return answers


def main():
    binary = os.path.abspath(sys.argv[1])
    scratch = os.path.abspath(sys.argv[2])
    os.makedirs(scratch)
    store = os.path.join(scratch, "wb", ".dinah", WORKBENCH_ID)
    shutil.copytree(FIXTURE, store)
    env = {k: v for k, v in os.environ.items() if not k.startswith("DINAH_")}
    env["DINAH_HOME"] = os.path.join(scratch, "home")
    env["DINAH_ACTOR"] = "sam"

    rows = []

    def capture(args, name):
        code, out, err = run(binary, store, env, args)
        rows.append({"args": args, "exit": code,
                     "stdout": canonical_item_keys(normalise(out, store)), "stderr": normalise(err, store)})

    # The members every card holds, counted from the machine answer of its
    # own show so that no position is guessed.
    cards = ["fx-1", "fx-3", "fx-4"]
    for card in cards:
        capture(["show", card, "--json"], card)
        capture(["show", card, "--json", "--all"], card)
        code, out, _ = run(binary, store, env, ["show", card, "--json", "--all"])
        whole = json.loads(out)
        for n, comment in enumerate(whole.get("comments") or [], start=1):
            ref = "%s/comments/%d" % (card, n)
            capture(["show", ref, "--json"], ref)
            capture(["show", ref], ref)
            for a, _ in enumerate(comment.get("attachments") or [], start=1):
                capture(["show", "%s/attachments/%d" % (ref, a), "--json"], ref)
                capture(["path", "%s/attachments/%d/payload" % (ref, a)], ref)
        for n, item in enumerate(whole.get("checklist") or [], start=1):
            ref = "%s/checklist/%d" % (card, n)
            capture(["show", ref, "--json"], ref)
            code, out, _ = run(binary, store, env, ["show", ref, "--json"])
            for c, comment in enumerate(json.loads(out).get("comments") or [], start=1):
                cref = "%s/comments/%d" % (ref, c)
                capture(["show", cref, "--json"], cref)
                capture(["show", cref], cref)
                for a, _ in enumerate(comment.get("attachments") or [], start=1):
                    capture(["show", "%s/attachments/%d" % (cref, a), "--json"], cref)
                    capture(["path", "%s/attachments/%d/payload" % (cref, a)], cref)

    # The archived half of the main card.
    for ref in ["fx-1/comments/1", "fx-1/checklist/1", "fx-1/checklist/1/comments/1",
                "fx-1/questions/2/comments/1"]:
        capture(["show", ref, "--archived", "--json"], ref)
        capture(["show", ref, "--archived"], ref)

    # Column comments, live and archived.
    capture(["show", "review/comments/1", "--json"], "review")
    capture(["show", "review/comments/1"], "review")
    capture(["show", "parked/comments/1", "--archived", "--json"], "parked")

    # The archived card, whole, and its members in both halves.
    code, out, _ = run(binary, store, env, ["list", "--archived", "--json"])
    capture(["list", "--archived", "--json"], "archived")
    archived_id = None
    for line in json.loads(out).get("cards", []) if isinstance(json.loads(out), dict) else []:
        archived_id = line.get("id")
    archived_id = archived_id or [d for d in os.listdir(os.path.join(store, "archive", "cards"))][0]
    capture(["show", "--archived", archived_id, "--all"], "archived")
    capture(["show", "--archived", archived_id, "--all", "--json"], "archived")
    for ref in ["%s/comments/1" % archived_id, "%s/checklist/1" % archived_id,
                "%s/criteria/1" % archived_id, "%s/checklist/1/comments/1" % archived_id]:
        capture(["show", ref, "--json"], ref)
        capture(["show", ref, "--archived", "--json"], ref)

    # The short forms and the kind-narrowed collections.
    for ref in ["fx-1/ac/1", "fx-1/oq/1", "fx-1/d/1", "fx-1/criteria/2", "fx-1/questions/2",
                "fx-1/decisions/1", "fx-1/oq/2/comments/2", "fx-1/criteria/3/comments/1"]:
        capture(["show", ref, "--json"], ref)

    # Identifier selectors, read from the old layout's own directories, and a
    # comment of an archived item addressed in the live half and the archived.
    main_card = json.loads(run(binary, store, env, ["show", "fx-1", "--json"])[1])["card"]["id"]
    card_dir = os.path.join(store, "cards", main_card)
    for comment_id in sorted(os.listdir(os.path.join(card_dir, "comments"))):
        capture(["show", "fx-1/comments/%s" % comment_id, "--json"], comment_id)
    for item_id in sorted(os.listdir(os.path.join(card_dir, "checklist"))):
        capture(["show", "fx-1/checklist/%s" % item_id, "--json"], item_id)
    for item_id in sorted(os.listdir(os.path.join(card_dir, "archive", "checklist"))):
        capture(["show", "fx-1/checklist/%s/comments/1" % item_id, "--json"], item_id)
        capture(["show", "fx-1/checklist/%s/comments/1" % item_id, "--archived", "--json"], item_id)
        capture(["show", "fx-1/checklist/%s" % item_id, "--archived", "--json"], item_id)
    column_dir = os.path.join(store, "columns", "3f9c17ade5b2", "comments")
    for comment_id in sorted(os.listdir(column_dir)):
        capture(["show", "review/comments/%s" % comment_id, "--json"], comment_id)

    # The workbench-wide reads.
    capture(["list"], "list")
    capture(["list", "--json"], "list")
    capture(["status"], "status")
    capture(["status", "--json"], "status")
    capture(["view", "board", "--plain"], "view")
    capture(["tree"], "tree")
    capture(["search", "vendor"], "search")
    capture(["search", "vendor", "--json"], "search")
    capture(["export"], "export")

    mcp = mcp_show(binary, store, env, ["fx-1", "fx-1/questions/2"])
    for ref, text in zip(["fx-1", "fx-1/questions/2"], mcp):
        rows.append({"mcp_show": ref, "stdout": canonical_item_keys(normalise(text, store))})

    os.makedirs(GOLDEN, exist_ok=True)
    with open(os.path.join(GOLDEN, "golden.json"), "w", newline="\n", encoding="utf-8") as out:
        json.dump(rows, out, indent=1, ensure_ascii=False)
        out.write("\n")
    print("captured %d rows" % len(rows))


if __name__ == "__main__":
    main()
