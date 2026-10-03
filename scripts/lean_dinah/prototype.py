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
BUILDER_ROLE = "You are the builder. Implement the task contract in the current workspace. Inspect the existing files, make the smallest complete change, run relevant checks, and record the checks in RESULT.md."
REVIEWER_ROLE = "You are the independent reviewer. Inspect the workspace beyond the builder report, evaluate the task contract and changed files, and run relevant checks. Do not edit the implementation. Write REVIEW.md with VERDICT: PASS or VERDICT: REPAIR on its first line, followed by findings and evidence."


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


def manifest_revision(root: Path) -> str:
    return digest_bytes(canonical(tree_manifest(root)))


def implementation_revision(root: Path) -> str:
    manifest = tree_manifest(root)
    manifest.pop("RESULT.md", None)
    manifest.pop("REVIEW.md", None)
    return digest_bytes(canonical(manifest))


def acceptance_file(fixture: Path, manifest: dict[str, Any]) -> Path:
    for part in manifest["acceptance_command"]:
        if "{fixture}" in part:
            return Path(part.replace("{fixture}", str(fixture)))
    raise Refused("acceptance command has no fixture-owned evaluator")


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
        self.executable = executable
        self.workbench = workbench
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


def packet_fresh(packet: dict[str, Any], revisions: dict[str, str], changes: dict[str, Any], check_code: bool = True) -> tuple[bool, str]:
    pairs = [("instructions_sha256", "instructions"), ("workbench_revision", "workbench"), ("card_revision", "card")]
    if check_code:
        pairs.append(("starting_code_revision", "code"))
    for packet_name, current_name in pairs:
        if packet[packet_name] != revisions[current_name]:
            return False, "%s changed" % current_name
    if changes.get("changed") or changes.get("events") or changes.get("changes"):
        return False, "authoritative state changed after the packet cursor"
    return True, "fresh"


def _card_value(result: dict[str, Any]) -> dict[str, Any]:
    detail = result.get("detail", result)
    return detail.get("card", detail)


def read_authority(session: DinahSession, card_ref: str, workspace: Path, since: str | None = None) -> tuple[dict[str, str], dict[str, Any]]:
    reader = DinahSession(session.executable, session.workbench)
    try:
        instructions = reader.call("instructions", {"card": card_ref})
        shown = reader.call("show", {"card": card_ref, "fields": "card"})
        workbench = reader.call("workbench", {})
        change_args = {"card": card_ref}
        if since is not None:
            change_args["since"] = since
        changes = reader.call("changes", change_args)
    finally:
        reader.close()
    card = _card_value(shown)
    if not card.get("revision") or not changes.get("cursor"):
        raise Refused("authoritative card revision or change cursor is absent")
    revisions = {
        "instructions": digest_bytes(canonical(instructions.get("instructions", instructions.get("served", {}).get("instructions", instructions)))),
        "code": manifest_revision(workspace),
        "workbench": digest_bytes(canonical(workbench)),
        "card": card["revision"],
        "cursor": changes["cursor"],
    }
    return revisions, changes


def require_fresh(packet: dict[str, Any], session: DinahSession, card_ref: str, workspace: Path, evidence_path: Path, gate: str, check_code: bool = True) -> dict[str, str]:
    revisions, changes = read_authority(session, card_ref, workspace, packet["changes_cursor"])
    fresh, reason = packet_fresh(packet, revisions, changes, check_code)
    record = {"gate": gate, "fresh": fresh, "reason": reason, "packet_sha256": packet["packet_sha256"], "observed": revisions, "changes": changes}
    write_json(evidence_path, record)
    if not fresh:
        raise Refused("%s freshness gate refused: %s" % (gate, reason))
    return revisions


def transition_request(card: str, destination: str, observed_revision: str, current_revision: str, allowed: list[str], operator: bool = False) -> dict[str, str]:
    if observed_revision != current_revision:
        raise Refused("stale card revision")
    if destination == "Done" and not operator:
        raise Refused("operator Acceptance is required before Done")
    if destination not in allowed:
        raise Refused("Dinah did not offer transition to %s" % destination)
    return {"card": card, "column": destination, "basis": observed_revision}


def evidence_receipt(workspace: Path, task: bytes, acceptance: bytes, argv: list[str], config: dict[str, Any], status: int, output: bytes, starting_revision: str) -> dict[str, Any]:
    return {
        "starting_revision": starting_revision,
        "merge_candidate_sha256": manifest_revision(workspace),
        "task_sha256": digest_bytes(task),
        "acceptance_sha256": digest_bytes(acceptance),
        "argv": argv,
        "config_sha256": digest_bytes(canonical(config)),
        "exit_status": status,
        "output_sha256": digest_bytes(output),
    }


def evidence_state(receipt: dict[str, Any], expected: dict[str, Any] | None) -> str:
    if expected is None:
        return "unknown"
    return "valid" if receipt == expected else "stale"


def check_state(exit_statuses: list[int]) -> str:
    if not exit_statuses:
        return "unknown"
    return "pass" if all(status == 0 for status in exit_statuses) else "fail"


def run_codex(executable: str, model: str, workspace: Path, prompt: bytes, evidence: Path, run_id: str, task: bytes, acceptance: bytes, command_facts: dict[str, str], role: str, timeout: int = 600) -> dict[str, Any]:
    argv = codex_command(executable, model, workspace)
    starting_revision = manifest_revision(workspace)
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
    record = {
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
    receipt = evidence_receipt(workspace, task, acceptance, argv, {"model": model, "command": command_facts, "role_sha256": digest_bytes(role.encode()), "prompt_sha256": digest_bytes(prompt)}, exit_status, transcript.read_bytes(), starting_revision)
    receipt_path = evidence / (run_id + ".receipt.json")
    write_json(receipt_path, receipt)
    record["receipt"] = str(receipt_path)
    return record


def run_acceptance(fixture: Path, workspace: Path, manifest: dict[str, Any], evidence: Path, run_id: str, provenance: dict[str, Any] | None = None) -> dict[str, Any]:
    command = [part.replace("{python}", sys.executable).replace("{workspace}", str(workspace)).replace("{fixture}", str(fixture)) for part in manifest["acceptance_command"]]
    starting_revision = manifest_revision(workspace)
    done = subprocess.run(command, capture_output=True, check=False)
    output = done.stdout + done.stderr
    output_path = evidence / (run_id + ".output.txt")
    output_path.write_bytes(output)
    task = (fixture / manifest["task_file"]).read_bytes()
    acceptance_path = acceptance_file(fixture, manifest)
    acceptance = acceptance_path.read_bytes()
    receipt = evidence_receipt(workspace, task, acceptance, command, {"kind": "external-acceptance", "provenance": provenance or {"kind": "contemporaneous"}}, done.returncode, output, starting_revision)
    receipt_path = evidence / (run_id + ".receipt.json")
    write_json(receipt_path, receipt)
    return {
        "argv": command,
        "exit_status": done.returncode,
        "output": output.decode("utf-8", "replace"),
        "output_path": str(output_path),
        "receipt": str(receipt_path),
    }


def builder_prompt(contract: str, lean: bool, packet: dict[str, Any] | None) -> bytes:
    coordination = "No Dinah coordination packet applies to this arm."
    if lean:
        coordination = "DINAH PACKET\n" + json.dumps(packet, indent=2)
    lead = BUILDER_ROLE + "\n\nCOORDINATION\n" + coordination + "\n\n"
    return (lead + "TASK CONTRACT\n" + contract).encode()


def reviewer_prompt(contract: str, lean: bool, packet: dict[str, Any] | None) -> bytes:
    coordination = "No Dinah coordination packet applies to this arm."
    if lean:
        coordination = "DINAH PACKET\n" + json.dumps(packet, indent=2)
    lead = REVIEWER_ROLE + "\n\nCOORDINATION\n" + coordination + "\n\n"
    return (lead + "TASK CONTRACT\n" + contract).encode()


def review_verdict(workspace: Path) -> tuple[str, str]:
    path = workspace / "REVIEW.md"
    if not path.exists():
        return "unknown", "REVIEW.md is absent"
    text = path.read_text(encoding="utf-8")
    first = text.splitlines()[0].strip() if text.splitlines() else ""
    if first == "VERDICT: PASS":
        return "pass", text
    if first == "VERDICT: REPAIR":
        return "repair", text
    return "unknown", text


def recovery_prompt(contract: str, findings: str, prior_result: str, packet: dict[str, Any] | None) -> bytes:
    coordination = {"kind": "explicit-recovery", "findings": findings, "prior_result": prior_result, "packet": packet}
    return (BUILDER_ROLE + "\n\nCOORDINATION\n" + json.dumps(coordination, indent=2) + "\n\nTASK CONTRACT\n" + contract).encode()


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
            task_bytes = contract.encode()
            acceptance_path = acceptance_file(fixture, manifest)
            acceptance_bytes = acceptance_path.read_bytes()
            packet = None
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
                revisions, _ = read_authority(session, card["ref"], workspace)
                packet = make_packet(contract, "external acceptance evaluator", "current workspace", revisions, ["worker checks", "external acceptance"])
                require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-builder-dispatch-gate.json" % fixture_name), "builder-dispatch")
            run_id = "%s-%s-builder" % (fixture_name, arm)
            builder = run_codex(executable, model, workspace, builder_prompt(contract, arm == "lean", packet), evidence, run_id, task_bytes, acceptance_bytes, command_facts, BUILDER_ROLE)
            ledger.import_run(run_id, builder["identity"], Usage(**builder["usage"]) if builder["usage"] else None, builder["usage_state"])
            runs = [builder]
            if arm != "direct" and builder["exit_status"] == 0:
                if arm == "lean":
                    require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-builder-result-gate.json" % fixture_name), "builder-result", check_code=False)
                    reviewed = session.call("move", {"card": card["ref"], "column": "review", "basis": card["revision"]})
                    card = reviewed["card"]
                    lifecycle.append({"act": "move", "column": card["column_title"], "revision": card["revision"]})
                    released = session.call("release", {"card": card["ref"], "basis": card["revision"]})
                    card = released["card"]
                    lifecycle.append({"act": "release-builder", "column": card["column_title"], "revision": card["revision"]})
                    claimed = session.call("claim", {"card": card["ref"], "basis": card["revision"], "expires": "2h", "actor": "lean-reviewer"})
                    card = claimed["card"]
                    lifecycle.append({"act": "claim-reviewer", "column": card["column_title"], "revision": card["revision"]})
                    revisions, _ = read_authority(session, card["ref"], workspace)
                    packet = make_packet(contract, "external acceptance evaluator", "current workspace", revisions, ["independent review checks", "external acceptance"])
                    require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-review-dispatch-gate.json" % fixture_name), "review-dispatch")
                review_id = "%s-%s-review" % (fixture_name, arm)
                before_review = implementation_revision(workspace)
                review = run_codex(executable, model, workspace, reviewer_prompt(contract, arm == "lean", packet), evidence, review_id, task_bytes, acceptance_bytes, command_facts, REVIEWER_ROLE)
                ledger.import_run(review_id, review["identity"], Usage(**review["usage"]) if review["usage"] else None, review["usage_state"])
                runs.append(review)
                if implementation_revision(workspace) != before_review:
                    raise Refused("reviewer changed implementation files")
                if arm == "lean" and review["exit_status"] == 0:
                    require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-review-result-gate.json" % fixture_name), "review-result", check_code=False)
                verdict, findings = review_verdict(workspace)
                if review["exit_status"] == 0 and verdict == "repair":
                    if arm == "lean":
                        released = session.call("release", {"card": card["ref"], "basis": card["revision"], "actor": "lean-reviewer"})["card"]
                        card = session.call("claim", {"card": released["ref"], "basis": released["revision"], "expires": "2h"})["card"]
                        revisions, _ = read_authority(session, card["ref"], workspace)
                        packet = make_packet(contract, "external acceptance evaluator", "current workspace", revisions, ["review findings", "worker checks"])
                        require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-recovery-dispatch-gate.json" % fixture_name), "recovery-dispatch")
                    result_text = (workspace / "RESULT.md").read_text(encoding="utf-8") if (workspace / "RESULT.md").exists() else "RESULT.md absent"
                    recovery_id = "%s-%s-recovery" % (fixture_name, arm)
                    recovery = run_codex(executable, model, workspace, recovery_prompt(contract, findings, result_text, packet if arm == "lean" else None), evidence, recovery_id, task_bytes, acceptance_bytes, command_facts, BUILDER_ROLE)
                    ledger.import_run(recovery_id, recovery["identity"], Usage(**recovery["usage"]) if recovery["usage"] else None, recovery["usage_state"])
                    runs.append(recovery)
                    if arm == "lean":
                        require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-recovery-result-gate.json" % fixture_name), "recovery-result", check_code=False)
                        released = session.call("release", {"card": card["ref"], "basis": card["revision"]})["card"]
                        card = session.call("claim", {"card": released["ref"], "basis": released["revision"], "expires": "2h", "actor": "lean-reviewer"})["card"]
                        revisions, _ = read_authority(session, card["ref"], workspace)
                        packet = make_packet(contract, "external acceptance evaluator", "current workspace", revisions, ["independent re-review", "external acceptance"])
                        require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-rereview-dispatch-gate.json" % fixture_name), "rereview-dispatch")
                    rereview_id = "%s-%s-rereview" % (fixture_name, arm)
                    before_review = implementation_revision(workspace)
                    rereview = run_codex(executable, model, workspace, reviewer_prompt(contract, arm == "lean", packet), evidence, rereview_id, task_bytes, acceptance_bytes, command_facts, REVIEWER_ROLE)
                    ledger.import_run(rereview_id, rereview["identity"], Usage(**rereview["usage"]) if rereview["usage"] else None, rereview["usage_state"])
                    runs.append(rereview)
                    if implementation_revision(workspace) != before_review:
                        raise Refused("reviewer changed implementation files")
                    if arm == "lean":
                        require_fresh(packet, session, card["ref"], workspace, evidence / ("%s-lean-rereview-result-gate.json" % fixture_name), "rereview-result", check_code=False)
                    verdict, findings = review_verdict(workspace)
                if verdict != "pass":
                    acceptance = {"argv": [], "exit_status": 2, "output": "review did not reach PASS: %s" % verdict, "output_path": None, "receipt": None}
                else:
                    acceptance = run_acceptance(fixture, workspace, manifest, evidence, "%s-%s-acceptance" % (fixture_name, arm))
            else:
                acceptance = run_acceptance(fixture, workspace, manifest, evidence, "%s-%s-acceptance" % (fixture_name, arm))
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


def run_integration(root: Path, dinah: str) -> dict[str, Any]:
    evidence = root / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    workspace = root / "workspace"
    workspace.mkdir(parents=True)
    (workspace / "seed.txt").write_text("seed\n", encoding="utf-8")
    workbench, session = create_candidate_workbench(dinah, root)
    result: dict[str, Any] = {"workbench": str(workbench)}
    try:
        added = session.call("add_card", {"title": "integration freshness task", "column": "ready"})["card"]
        moved = session.call("move", {"card": added["ref"], "column": "working", "basis": added["revision"]})["card"]
        card = session.call("claim", {"card": moved["ref"], "basis": moved["revision"], "expires": "2h"})["card"]
        revisions, _ = read_authority(session, card["ref"], workspace)
        packet = make_packet("integration task", "integration acceptance", "workspace", revisions, ["receipt"])
        require_fresh(packet, session, card["ref"], workspace, evidence / "valid-dispatch-gate.json", "valid-dispatch")

        starting = manifest_revision(workspace)
        (workspace / "candidate.txt").write_text("candidate\n", encoding="utf-8")
        receipt = evidence_receipt(workspace, b"integration task", b"integration acceptance", ["stub-worker"], {"role_sha256": digest_bytes(BUILDER_ROLE.encode())}, 0, b"stub output", starting)
        write_json(evidence / "stub-worker.receipt.json", receipt)
        same = evidence_receipt(workspace, b"integration task", b"integration acceptance", ["stub-worker"], {"role_sha256": digest_bytes(BUILDER_ROLE.encode())}, 0, b"stub output", starting)
        valid_receipt = evidence_state(receipt, same)
        (workspace / "candidate.txt").write_text("changed\n", encoding="utf-8")
        changed = evidence_receipt(workspace, b"integration task", b"integration acceptance", ["stub-worker"], {"role_sha256": digest_bytes(BUILDER_ROLE.encode())}, 0, b"stub output", starting)
        stale_receipt = evidence_state(receipt, changed)

        released = session.call("release", {"card": card["ref"], "basis": card["revision"]})["card"]
        session.call("claim", {"card": released["ref"], "basis": released["revision"], "expires": "2h", "actor": "late-actor"})
        try:
            require_fresh(packet, session, card["ref"], workspace, evidence / "stale-result-gate.json", "stale-result", check_code=False)
            stale_gate = "accepted"
        except Refused:
            stale_gate = "refused"

        recovery_card = session.call("add_card", {"title": "integration recovery task", "column": "ready"})["card"]
        recovery_card = session.call("move", {"card": recovery_card["ref"], "column": "working", "basis": recovery_card["revision"]})["card"]
        recovery_card = session.call("claim", {"card": recovery_card["ref"], "basis": recovery_card["revision"], "expires": "2h"})["card"]
        recovery_card = session.call("move", {"card": recovery_card["ref"], "column": "review", "basis": recovery_card["revision"]})["card"]
        recovery_card = session.call("release", {"card": recovery_card["ref"], "basis": recovery_card["revision"]})["card"]
        recovery_card = session.call("claim", {"card": recovery_card["ref"], "basis": recovery_card["revision"], "expires": "2h", "actor": "lean-reviewer"})["card"]
        before = implementation_revision(workspace)
        (workspace / "REVIEW.md").write_text("VERDICT: REPAIR\nFix the candidate value.\n", encoding="utf-8")
        if implementation_revision(workspace) != before or review_verdict(workspace)[0] != "repair":
            raise Refused("read-only review integration failed")
        recovery_card = session.call("release", {"card": recovery_card["ref"], "basis": recovery_card["revision"], "actor": "lean-reviewer"})["card"]
        recovery_card = session.call("claim", {"card": recovery_card["ref"], "basis": recovery_card["revision"], "expires": "2h"})["card"]
        revisions, _ = read_authority(session, recovery_card["ref"], workspace)
        recovery_packet = make_packet("integration task", "integration acceptance", "workspace", revisions, ["review findings"])
        require_fresh(recovery_packet, session, recovery_card["ref"], workspace, evidence / "recovery-dispatch-gate.json", "recovery-dispatch")
        recovery_bytes = recovery_prompt("integration task", "Fix the candidate value.", "prior result", recovery_packet)
        (workspace / "candidate.txt").write_text("repaired\n", encoding="utf-8")
        require_fresh(recovery_packet, session, recovery_card["ref"], workspace, evidence / "recovery-result-gate.json", "recovery-result", check_code=False)
        recovery_card = session.call("release", {"card": recovery_card["ref"], "basis": recovery_card["revision"]})["card"]
        recovery_card = session.call("claim", {"card": recovery_card["ref"], "basis": recovery_card["revision"], "expires": "2h", "actor": "lean-reviewer"})["card"]
        (workspace / "REVIEW.md").write_text("VERDICT: PASS\nRepair verified.\n", encoding="utf-8")

        instruction_card = session.call("add_card", {"title": "instruction change task", "column": "ready"})["card"]
        instruction_card = session.call("move", {"card": instruction_card["ref"], "column": "working", "basis": instruction_card["revision"]})["card"]
        instruction_card = session.call("claim", {"card": instruction_card["ref"], "basis": instruction_card["revision"], "expires": "2h"})["card"]
        revisions, _ = read_authority(session, instruction_card["ref"], workspace)
        instruction_packet = make_packet("instruction task", "accept", "workspace", revisions, [])
        working_column = session.workbench / "columns" / "222222222222" / "column.md"
        working_column.write_text(working_column.read_text(encoding="utf-8") + "\nChanged instruction for the integration refusal check.\n", encoding="utf-8")
        try:
            require_fresh(instruction_packet, session, instruction_card["ref"], workspace, evidence / "changed-instruction-gate.json", "changed-instruction")
            instruction_gate = "accepted"
        except Refused:
            instruction_gate = "refused"
        revisions, _ = read_authority(session, instruction_card["ref"], workspace)
        refreshed_packet = make_packet("instruction task", "accept", "workspace", revisions, [])
        require_fresh(refreshed_packet, session, instruction_card["ref"], workspace, evidence / "refreshed-instruction-gate.json", "refreshed-instruction")

        session.call("comment", {"card": instruction_card["ref"], "text": "Late decision: require the candidate value."})
        try:
            require_fresh(refreshed_packet, session, instruction_card["ref"], workspace, evidence / "late-decision-gate.json", "late-decision")
            late_decision_gate = "accepted"
        except Refused:
            late_decision_gate = "refused"
        revisions, _ = read_authority(session, instruction_card["ref"], workspace)
        decision_packet = make_packet("instruction task", "accept", "workspace", revisions, [])
        require_fresh(decision_packet, session, instruction_card["ref"], workspace, evidence / "refreshed-decision-gate.json", "refreshed-decision")
        result.update({"valid_gate": "accepted", "stale_gate": stale_gate, "instruction_change_gate": instruction_gate, "late_decision_gate": late_decision_gate, "valid_receipt": valid_receipt, "changed_candidate_receipt": stale_receipt, "recovery_path": review_verdict(workspace)[0], "recovery_prompt_sha256": digest_bytes(recovery_bytes), "builder_role_sha256": digest_bytes(BUILDER_ROLE.encode()), "reviewer_role_sha256": digest_bytes(REVIEWER_ROLE.encode())})
        write_json(root / "integration-report.json", result)
        if (valid_receipt, stale_receipt, stale_gate, instruction_gate, late_decision_gate, result["recovery_path"]) != ("valid", "stale", "refused", "refused", "refused", "pass"):
            raise Refused("integration dry run did not enforce the expected gates")
        return result
    finally:
        session.close()


def run_preflight(root: Path, model: str, executable: str) -> dict[str, Any]:
    command_facts = check_codex(executable)
    workspace = root / "write-preflight"
    if workspace.exists():
        raise Refused("preflight workspace already exists: %s" % workspace)
    workspace.mkdir(parents=True)
    evidence = root / "evidence"
    evidence.mkdir(parents=True, exist_ok=True)
    prompt = b"Create exactly one new file named ready.txt containing only READY and a newline. Do not inspect anything else. Then stop."
    record = run_codex(executable, model, workspace, prompt, evidence, "write-preflight", prompt, b"ready.txt must contain READY newline", command_facts, "write readiness probe", timeout=180)
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
    integration = sub.add_parser("integration")
    integration.add_argument("--root", required=True, type=Path)
    integration.add_argument("--dinah", default="dinah")
    acceptance = sub.add_parser("acceptance")
    acceptance.add_argument("--fixture", required=True, type=Path)
    acceptance.add_argument("--workspace", required=True, type=Path)
    acceptance.add_argument("--evidence", required=True, type=Path)
    acceptance.add_argument("--run-id", required=True)
    acceptance.add_argument("--source-campaign", required=True)
    acceptance.add_argument("--worker-transcript", type=Path)
    args = parser.parse_args()
    try:
        if args.command == "preflight":
            report = run_preflight(args.root.resolve(), args.model, args.codex)
        elif args.command == "integration":
            report = run_integration(args.root.resolve(), args.dinah)
        elif args.command == "acceptance":
            manifest = json.loads((args.fixture / "fixture.json").read_text(encoding="utf-8"))
            args.evidence.mkdir(parents=True, exist_ok=True)
            provenance = {"kind": "post-hoc-reconstruction", "source_campaign": args.source_campaign, "worker_transcript_sha256": file_hash(args.worker_transcript) if args.worker_transcript else None}
            report = run_acceptance(args.fixture.resolve(), args.workspace.resolve(), manifest, args.evidence.resolve(), args.run_id, provenance)
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
