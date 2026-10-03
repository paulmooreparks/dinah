#!/usr/bin/env python3
"""Run and audit the bounded lean Dinah prompting pilot."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import time
import uuid
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any


REQUIRED_HELP = (
    "--ignore-user-config",
    "--sandbox <SANDBOX_MODE>",
    "--model <MODEL>",
    "--json",
    "--ephemeral",
    "--skip-git-repo-check",
    "--cd <DIR>",
)
ARMS = ("direct", "direct-review", "lean")
MCP_VERSION = "2024-11-05"


class Refused(RuntimeError):
    """The prototype stopped because a required condition was absent."""


def digest_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def canonical(value: Any) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode()


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(json.dumps(value, indent=2, sort_keys=True).encode() + b"\n")


def file_hash(path: Path) -> str:
    return digest_bytes(path.read_bytes())


def tree_manifest(root: Path) -> dict[str, str]:
    result = {}
    for path in sorted(p for p in root.rglob("*") if p.is_file()):
        if ".git" not in path.parts:
            result[path.relative_to(root).as_posix()] = file_hash(path)
    return result


def codex_command(executable: str, model: str, workspace: Path, ephemeral: bool = True) -> list[str]:
    argv = [
        executable,
        "exec",
        "--ignore-user-config",
        "-c",
        'windows.sandbox="elevated"',
        "--sandbox",
        "workspace-write",
        "-m",
        model,
        "--json",
    ]
    if ephemeral:
        argv.append("--ephemeral")
    argv.extend(["--skip-git-repo-check", "-C", str(workspace), "-"])
    return argv


def check_codex(executable: str) -> dict[str, str]:
    version = subprocess.run(
        [executable, "--version"], capture_output=True, text=True, check=False
    )
    help_run = subprocess.run(
        [executable, "exec", "--help"], capture_output=True, text=True, check=False
    )
    if version.returncode or help_run.returncode:
        raise Refused("codex version or exec help failed")
    missing = [flag for flag in REQUIRED_HELP if flag not in help_run.stdout]
    if missing:
        raise Refused("codex exec help lacks required text: " + ", ".join(missing))
    return {"version": version.stdout.strip(), "help_sha256": digest_bytes(help_run.stdout.encode())}


@dataclass(frozen=True)
class Usage:
    input_tokens: int
    cached_input_tokens: int
    output_tokens: int
    reasoning_output_tokens: int | None = None
    cache_write_input_tokens: int | None = None


def terminal_usage(transcript: Path, successful: bool) -> tuple[str | None, Usage | None, str]:
    if not successful:
        return None, None, "incomplete: process failed"
    terminal = []
    thread_id = None
    try:
        for raw in transcript.read_text(encoding="utf-8").splitlines():
            event = json.loads(raw)
            thread_id = event.get("thread_id", thread_id)
            if event.get("type") == "turn.completed":
                terminal.append(event.get("usage"))
    except (OSError, json.JSONDecodeError) as exc:
        return thread_id, None, "incomplete: invalid JSON Lines: %s" % exc
    if len(terminal) != 1 or not isinstance(terminal[0], dict):
        return thread_id, None, "incomplete: expected one terminal usage event, found %d" % len(terminal)
    value = terminal[0]
    required = ("input_tokens", "cached_input_tokens", "output_tokens")
    if any(not isinstance(value.get(name), int) for name in required):
        return thread_id, None, "incomplete: terminal usage lacks required categories"
    return thread_id, Usage(
        input_tokens=value["input_tokens"],
        cached_input_tokens=value["cached_input_tokens"],
        output_tokens=value["output_tokens"],
        reasoning_output_tokens=value.get("reasoning_output_tokens"),
        cache_write_input_tokens=value.get("cache_write_input_tokens"),
    ), "complete"


class UsageLedger:
    def __init__(self, path: Path):
        self.path = path
        self.rows = json.loads(path.read_text()) if path.exists() else {}

    def import_run(self, run_id: str, identity: dict[str, Any], usage: Usage | None, state: str) -> str:
        row = {"identity_sha256": digest_bytes(canonical(identity)), "usage": asdict(usage) if usage else None, "state": state}
        old = self.rows.get(run_id)
        if old is not None and old != row:
            raise Refused("run identifier %s has a conflicting duplicate" % run_id)
        self.rows[run_id] = row
        write_json(self.path, self.rows)
        return "duplicate" if old is not None else "recorded"


class DinahSession:
    def __init__(self, executable: str, workbench: Path):
        env = os.environ.copy()
        env.update({"DINAH_ACTOR": "lean-adapter", "DINAH_PROVIDER": "openai", "DINAH_MODEL": "gpt-5.6-sol", "DINAH_HARNESS": "codex"})
        self.process = subprocess.Popen(
            [executable, "--workbench", str(workbench), "mcp"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            encoding="utf-8",
            bufsize=1,
            env=env,
        )
        self.next_id = 0
        self.request("initialize", {"protocolVersion": MCP_VERSION, "capabilities": {}, "clientInfo": {"name": "lean-dinah-pilot", "version": "1"}})

    def request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        self.next_id += 1
        message = {"jsonrpc": "2.0", "id": self.next_id, "method": method, "params": params}
        self.process.stdin.write(json.dumps(message) + "\n")
        self.process.stdin.flush()
        line = self.process.stdout.readline()
        if not line:
            complaint = self.process.stderr.read()
            raise Refused("Dinah MCP closed before answering: %s" % complaint)
        answer = json.loads(line)
        if "error" in answer:
            raise Refused("Dinah MCP protocol error: %s" % answer["error"])
        return answer["result"]

    def call(self, name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        arguments = dict(arguments)
        arguments.setdefault("actor", "lean-adapter")
        arguments.setdefault("provider", "openai")
        arguments.setdefault("model", "gpt-5.6-sol")
        arguments.setdefault("harness", "codex")
        result = self.request("tools/call", {"name": name, "arguments": arguments})
        if result.get("isError"):
            raise Refused("Dinah refused %s: %s" % (name, result))
        text = result["content"][0]["text"]
        parsed = json.loads(text)
        if parsed.get("outcome") == "refused":
            raise Refused("Dinah refused %s: %s" % (name, parsed))
        return parsed

    def close(self) -> None:
        self.process.stdin.close()
        try:
            self.process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.process.kill()


def create_candidate_workbench(executable: str, root: Path) -> tuple[Path, DinahSession]:
    project = root / "candidate-workbench"
    template = Path(__file__).parent / "workbench"
    done = subprocess.run(
        [executable, "init", str(project), "--from", str(template), "--slug", "lean", "--operator", "paul"],
        capture_output=True,
        text=True,
        check=False,
    )
    if done.returncode:
        raise Refused("candidate workbench init failed: %s%s" % (done.stdout, done.stderr))
    stores = list((project / ".dinah").iterdir())
    if len(stores) != 1:
        raise Refused("candidate workbench init produced %d stores" % len(stores))
    return stores[0], DinahSession(executable, stores[0])


def make_packet(task: str, acceptance: str, scope: str, revisions: dict[str, str], evidence: list[str]) -> dict[str, Any]:
    packet = {
        "run_id": str(uuid.uuid4()),
        "attempt_id": str(uuid.uuid4()),
        "task": task,
        "scope": scope,
        "acceptance": acceptance,
        "decisions": [],
        "instructions_sha256": revisions["instructions"],
        "starting_code_revision": revisions["code"],
        "workbench_revision": revisions["workbench"],
        "card_revision": revisions["card"],
        "changes_cursor": revisions["cursor"],
        "evidence_requirements": evidence,
    }
    packet["packet_sha256"] = digest_bytes(canonical(packet))
    return packet


def packet_fresh(packet: dict[str, Any], revisions: dict[str, str], changes: list[dict[str, Any]]) -> tuple[bool, str]:
    pairs = (("instructions_sha256", "instructions"), ("starting_code_revision", "code"), ("workbench_revision", "workbench"), ("card_revision", "card"), ("changes_cursor", "cursor"))
    for packet_name, current_name in pairs:
        if packet[packet_name] != revisions[current_name]:
            return False, "%s changed" % current_name
    if any(change.get("affects_task") for change in changes):
        return False, "a late decision affects the task"
    return True, "fresh"


def transition_request(card: str, destination: str, observed_revision: str, current_revision: str, allowed: list[str], operator: bool = False) -> dict[str, str]:
    if observed_revision != current_revision:
        raise Refused("stale card revision")
    if destination == "Done" and not operator:
        raise Refused("operator Acceptance is required before Done")
    if destination not in allowed:
        raise Refused("Dinah did not offer transition to %s" % destination)
    return {"card": card, "column": destination, "basis": observed_revision}


def evidence_receipt(workspace: Path, task: bytes, acceptance: bytes, argv: list[str], config: dict[str, str], status: int, output: bytes) -> dict[str, Any]:
    manifest = tree_manifest(workspace)
    return {
        "repository_revision": config["repository_revision"],
        "dirty_manifest_sha256": digest_bytes(canonical(manifest)),
        "task_sha256": digest_bytes(task),
        "acceptance_sha256": digest_bytes(acceptance),
        "argv": argv,
        "config_sha256": digest_bytes(canonical(config)),
        "exit_status": status,
        "output_sha256": digest_bytes(output),
    }


def evidence_matches(receipt: dict[str, Any], expected: dict[str, Any]) -> bool:
    return receipt == expected


def run_codex(executable: str, model: str, workspace: Path, prompt: bytes, evidence: Path, run_id: str, timeout: int = 600) -> dict[str, Any]:
    argv = codex_command(executable, model, workspace)
    transcript = evidence / (run_id + ".jsonl")
    stderr = evidence / (run_id + ".stderr.txt")
    started = time.time()
    with transcript.open("wb") as out, stderr.open("wb") as err:
        try:
            done = subprocess.run(argv, input=prompt, stdout=out, stderr=err, check=False, timeout=timeout)
            exit_status = done.returncode
        except subprocess.TimeoutExpired:
            exit_status = -1
    finished = time.time()
    thread_id, usage, state = terminal_usage(transcript, exit_status == 0)
    if exit_status == -1:
        state = "incomplete: process timed out after %d seconds" % timeout
    identity = {
        "argv": argv,
        "prompt_sha256": digest_bytes(prompt),
        "workspace": str(workspace),
        "transcript_sha256": file_hash(transcript),
        "model": model,
    }
    return {
        "run_id": run_id,
        "argv": argv,
        "started": started,
        "finished": finished,
        "exit_status": exit_status,
        "thread_id": thread_id,
        "usage": asdict(usage) if usage else None,
        "usage_state": state,
        "identity": identity,
        "transcript": str(transcript),
    }


def run_acceptance(fixture: Path, workspace: Path, manifest: dict[str, Any]) -> dict[str, Any]:
    command = [part.replace("{python}", sys.executable).replace("{workspace}", str(workspace)).replace("{fixture}", str(fixture)) for part in manifest["acceptance_command"]]
    done = subprocess.run(command, capture_output=True, check=False)
    return {
        "argv": command,
        "exit_status": done.returncode,
        "output": done.stdout.decode("utf-8", "replace") + done.stderr.decode("utf-8", "replace"),
    }


def builder_prompt(contract: str, lean: bool, packet: dict[str, Any] | None) -> bytes:
    lead = "Implement the task in the current workspace. Run your own relevant checks.\n\n"
    if lean:
        lead = "You are the builder. Implement only the supplied packet in the current workspace. Record the checks you ran in RESULT.md.\n\nPACKET\n" + json.dumps(packet, indent=2) + "\n\n"
    return (lead + "TASK CONTRACT\n" + contract).encode()


def reviewer_prompt(contract: str, lean: bool, packet: dict[str, Any] | None) -> bytes:
    lead = "Independently review the implementation in this workspace against the task contract. Inspect the files and run relevant checks. Fix every defect you find, then record findings and repairs in REVIEW.md.\n\n"
    if lean:
        lead = "You are the independent reviewer. Inspect the workspace beyond the builder report. Evaluate the contract, diff, and evidence. Fix every defect you find and record findings and repairs in REVIEW.md.\n\nPACKET\n" + json.dumps(packet, indent=2) + "\n\n"
    return (lead + "TASK CONTRACT\n" + contract).encode()


def run_pilot(root: Path, model: str, executable: str, dinah: str, fixtures: list[str], skips: set[str]) -> dict[str, Any]:
    command_facts = check_codex(executable)
    source = Path(__file__).parent / "fixtures"
    evidence = root / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    ledger = UsageLedger(evidence / "usage-ledger.json")
    workbench, session = create_candidate_workbench(dinah, root)
    attempts = []
    try:
      for fixture_name in fixtures:
        fixture = source / fixture_name
        manifest = json.loads((fixture / "fixture.json").read_text(encoding="utf-8"))
        contract = (fixture / manifest["task_file"]).read_text(encoding="utf-8")
        arm_order = manifest.get("arm_order", list(ARMS))
        if sorted(arm_order) != sorted(ARMS):
            raise Refused("fixture %s has an invalid arm order" % fixture_name)
        for order, arm in enumerate(arm_order, 1):
            if "%s:%s" % (fixture_name, arm) in skips:
                continue
            workspace = root / "workspaces" / fixture_name / arm
            if workspace.exists():
                raise Refused("pilot workspace already exists: %s" % workspace)
            shutil.copytree(fixture / manifest["workspace_seed"], workspace)
            revisions = {"instructions": digest_bytes(b"builder-v1"), "code": digest_bytes(canonical(tree_manifest(workspace))), "workbench": "workbench-v1", "card": "card-v1", "cursor": "0"}
            packet = make_packet(contract, "external acceptance evaluator", "current workspace", revisions, ["worker checks", "external acceptance"])
            lifecycle = []
            if arm == "lean":
                added = session.call("add_card", {"title": "%s measured task" % fixture_name, "column": "ready"})
                card = added["card"]
                lifecycle.append({"act": "add", "column": card["column_title"], "revision": card["revision"]})
                working = session.call("move", {"card": card["ref"], "column": "working", "basis": card["revision"]})
                card = working["card"]
                lifecycle.append({"act": "move", "column": card["column_title"], "revision": card["revision"]})
                claimed = session.call("claim", {"card": card["ref"], "basis": card["revision"], "expires": "2h"})
                card = claimed["card"]
                lifecycle.append({"act": "claim-builder", "column": card["column_title"], "revision": card["revision"]})
            run_id = "%s-%s-builder" % (fixture_name, arm)
            builder = run_codex(executable, model, workspace, builder_prompt(contract, arm == "lean", packet if arm == "lean" else None), evidence, run_id)
            ledger.import_run(run_id, builder["identity"], Usage(**builder["usage"]) if builder["usage"] else None, builder["usage_state"])
            runs = [builder]
            if arm != "direct" and builder["exit_status"] == 0:
                if arm == "lean":
                    reviewed = session.call("move", {"card": card["ref"], "column": "review", "basis": card["revision"]})
                    card = reviewed["card"]
                    lifecycle.append({"act": "move", "column": card["column_title"], "revision": card["revision"]})
                    released = session.call("release", {"card": card["ref"], "basis": card["revision"]})
                    card = released["card"]
                    lifecycle.append({"act": "release-builder", "column": card["column_title"], "revision": card["revision"]})
                    claimed = session.call("claim", {"card": card["ref"], "basis": card["revision"], "expires": "2h", "actor": "lean-reviewer"})
                    card = claimed["card"]
                    lifecycle.append({"act": "claim-reviewer", "column": card["column_title"], "revision": card["revision"]})
                review_id = "%s-%s-review" % (fixture_name, arm)
                review = run_codex(executable, model, workspace, reviewer_prompt(contract, arm == "lean", packet if arm == "lean" else None), evidence, review_id)
                ledger.import_run(review_id, review["identity"], Usage(**review["usage"]) if review["usage"] else None, review["usage_state"])
                runs.append(review)
            acceptance = run_acceptance(fixture, workspace, manifest)
            if arm == "lean" and acceptance["exit_status"] == 0:
                released = session.call("release", {"card": card["ref"], "basis": card["revision"], "actor": "lean-reviewer"})
                card = released["card"]
                lifecycle.append({"act": "release-reviewer", "column": card["column_title"], "revision": card["revision"]})
                accepted = session.call("move", {"card": card["ref"], "column": "acceptance", "basis": card["revision"], "actor": "lean-reviewer"})
                card = accepted["card"]
                lifecycle.append({"act": "move", "column": card["column_title"], "revision": card["revision"]})
            attempts.append({"fixture": fixture_name, "arm": arm, "arm_order": order, "runs": runs, "acceptance": acceptance, "lifecycle": lifecycle})
            write_json(evidence / "attempts.json", attempts)
    finally:
        session.close()
    return {"command": command_facts, "dinah": {"executable": dinah, "workbench": str(workbench)}, "model": model, "attempts": attempts, "ledger": ledger.rows}


def run_preflight(root: Path, model: str, executable: str) -> dict[str, Any]:
    command_facts = check_codex(executable)
    workspace = root / "write-preflight"
    if workspace.exists():
        raise Refused("preflight workspace already exists: %s" % workspace)
    workspace.mkdir(parents=True)
    evidence = root / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    prompt = b"Create exactly one new file named ready.txt containing only READY and a newline. Do not inspect anything else. Then stop."
    record = run_codex(executable, model, workspace, prompt, evidence, "write-preflight", timeout=180)
    ready = workspace / "ready.txt"
    if record["exit_status"] != 0 or not ready.exists() or ready.read_bytes() != b"READY\n":
        write_json(root / "preflight.json", {"command": command_facts, "run": record, "ready": False})
        raise Refused("write preflight did not create ready.txt with the required bytes")
    report = {"command": command_facts, "run": record, "ready": True}
    write_json(root / "preflight.json", report)
    return report


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    pilot = sub.add_parser("pilot")
    pilot.add_argument("--root", required=True, type=Path)
    pilot.add_argument("--model", required=True)
    pilot.add_argument("--codex", default="codex")
    pilot.add_argument("--dinah", default="dinah")
    pilot.add_argument("--fixture", action="append", required=True)
    pilot.add_argument("--skip", action="append", default=[])
    preflight = sub.add_parser("preflight")
    preflight.add_argument("--root", required=True, type=Path)
    preflight.add_argument("--model", required=True)
    preflight.add_argument("--codex", default="codex")
    args = parser.parse_args()
    try:
        if args.command == "preflight":
            report = run_preflight(args.root.resolve(), args.model, args.codex)
        else:
            report = run_pilot(args.root.resolve(), args.model, args.codex, args.dinah, args.fixture, set(args.skip))
            write_json(args.root / "report.json", report)
        print(json.dumps(report, indent=2))
        return 0
    except Refused as exc:
        print("refused: %s" % exc, file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
