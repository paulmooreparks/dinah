import json
import tempfile
import unittest
from pathlib import Path

from scripts.lean_dinah import prototype


class UsageTests(unittest.TestCase):
    def test_terminal_usage_accepts_one_complete_event(self):
        with tempfile.TemporaryDirectory() as raw:
            path = Path(raw) / "run.jsonl"
            path.write_text(json.dumps({"type": "turn.completed", "thread_id": "t1", "usage": {"input_tokens": 10, "cached_input_tokens": 4, "output_tokens": 2}}) + "\n")
            thread, usage, state = prototype.terminal_usage(path, True)
            self.assertEqual((thread, state), ("t1", "complete"))
            self.assertEqual(usage.input_tokens, 10)
            self.assertEqual(usage.cached_input_tokens, 4)

    def test_terminal_usage_keeps_failure_unknown(self):
        with tempfile.TemporaryDirectory() as raw:
            path = Path(raw) / "run.jsonl"
            path.write_text("")
            _, usage, state = prototype.terminal_usage(path, False)
            self.assertIsNone(usage)
            self.assertIn("failed", state)

    def test_ledger_is_idempotent_and_rejects_conflict(self):
        with tempfile.TemporaryDirectory() as raw:
            ledger = prototype.UsageLedger(Path(raw) / "ledger.json")
            usage = prototype.Usage(10, 4, 2)
            self.assertEqual(ledger.import_run("r1", {"x": 1}, usage, "complete"), "recorded")
            self.assertEqual(ledger.import_run("r1", {"x": 1}, usage, "complete"), "duplicate")
            with self.assertRaises(prototype.Refused):
                ledger.import_run("r1", {"x": 2}, usage, "complete")


class FreshnessTests(unittest.TestCase):
    def setUp(self):
        self.revisions = {"instructions": "i", "code": "c", "workbench": "w", "card": "k", "cursor": "0"}
        self.packet = prototype.make_packet("task", "accept", "scope", self.revisions, ["test"])

    def test_fresh_packet_accepts(self):
        self.assertEqual(prototype.packet_fresh(self.packet, self.revisions, {"changed": False, "events": []}), (True, "fresh"))

    def test_instruction_change_refuses(self):
        changed = dict(self.revisions, instructions="later")
        self.assertEqual(prototype.packet_fresh(self.packet, changed, {"changed": False})[0], False)

    def test_late_decision_refuses(self):
        self.assertEqual(prototype.packet_fresh(self.packet, self.revisions, {"changed": True, "events": [{"kind": "decision"}]})[0], False)

    def test_result_gate_allows_worker_code_but_refuses_card_change(self):
        changed_code = dict(self.revisions, code="worker-result")
        self.assertTrue(prototype.packet_fresh(self.packet, changed_code, {"changed": False}, check_code=False)[0])
        changed_card = dict(changed_code, card="later")
        self.assertFalse(prototype.packet_fresh(self.packet, changed_card, {"changed": False}, check_code=False)[0])

    def test_transition_carries_basis_and_stops_on_stale_card(self):
        request = prototype.transition_request("card-1", "Review", "rev-1", "rev-1", ["Review"])
        self.assertEqual(request["basis"], "rev-1")
        with self.assertRaises(prototype.Refused):
            prototype.transition_request("card-1", "Review", "rev-1", "rev-2", ["Review"])

    def test_non_operator_cannot_leave_acceptance(self):
        with self.assertRaises(prototype.Refused):
            prototype.transition_request("card-1", "Done", "rev-1", "rev-1", ["Done"])


class EvidenceTests(unittest.TestCase):
    def test_receipt_accepts_unchanged_candidate_and_refuses_changed_candidate(self):
        with tempfile.TemporaryDirectory() as raw:
            workspace = Path(raw)
            (workspace / "one.txt").write_text("one")
            receipt = prototype.evidence_receipt(workspace, b"task", b"accept", ["check"], {"model": "m"}, 0, b"ok", "start")
            expected = prototype.evidence_receipt(workspace, b"task", b"accept", ["check"], {"model": "m"}, 0, b"ok", "start")
            self.assertEqual(prototype.evidence_state(receipt, expected), "valid")
            (workspace / "one.txt").write_text("changed")
            changed = prototype.evidence_receipt(workspace, b"task", b"accept", ["check"], {"model": "m"}, 0, b"ok", "start")
            self.assertEqual(prototype.evidence_state(receipt, changed), "stale")

    def test_changed_acceptance_invalidates_receipt(self):
        with tempfile.TemporaryDirectory() as raw:
            workspace = Path(raw)
            receipt = prototype.evidence_receipt(workspace, b"task", b"accept-v1", ["check"], {}, 0, b"ok", "start")
            changed = prototype.evidence_receipt(workspace, b"task", b"accept-v2", ["check"], {}, 0, b"ok", "start")
            self.assertEqual(prototype.evidence_state(receipt, changed), "stale")

    def test_changed_output_invalidates_receipt(self):
        with tempfile.TemporaryDirectory() as raw:
            workspace = Path(raw)
            receipt = prototype.evidence_receipt(workspace, b"task", b"accept", ["check"], {}, 0, b"first", "start")
            changed = prototype.evidence_receipt(workspace, b"task", b"accept", ["check"], {}, 0, b"second", "start")
            self.assertEqual(prototype.evidence_state(receipt, changed), "stale")

    def test_empty_check_result_remains_unknown(self):
        self.assertEqual(prototype.check_state([]), "unknown")
        self.assertEqual(prototype.check_state([0, 0]), "pass")
        self.assertEqual(prototype.check_state([0, 1]), "fail")


class PromptTests(unittest.TestCase):
    def test_role_instruction_is_identical_across_arms(self):
        packet = prototype.make_packet("task", "accept", "scope", {"instructions": "i", "code": "c", "workbench": "w", "card": "k", "cursor": "z"}, [])
        direct = prototype.builder_prompt("contract", False, None).decode()
        lean = prototype.builder_prompt("contract", True, packet).decode()
        self.assertTrue(direct.startswith(prototype.BUILDER_ROLE))
        self.assertTrue(lean.startswith(prototype.BUILDER_ROLE))
        self.assertEqual(prototype.digest_bytes(prototype.BUILDER_ROLE.encode()), prototype.digest_bytes(direct.split("\n\nCOORDINATION", 1)[0].encode()))


class CommandTests(unittest.TestCase):
    def test_command_pins_model_and_reads_stdin(self):
        command = prototype.codex_command("codex", "gpt-5.6-sol", Path("work"))
        self.assertEqual(command[-1], "-")
        self.assertIn("--ignore-user-config", command)
        self.assertEqual(command[command.index("-c") + 1], 'windows.sandbox="elevated"')
        self.assertEqual(command[command.index("-m") + 1], "gpt-5.6-sol")
        self.assertIn("--ephemeral", command)


if __name__ == "__main__":
    unittest.main()
