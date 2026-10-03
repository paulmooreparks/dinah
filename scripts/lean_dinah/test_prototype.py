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
        self.assertEqual(prototype.packet_fresh(self.packet, self.revisions, []), (True, "fresh"))

    def test_instruction_change_refuses(self):
        changed = dict(self.revisions, instructions="later")
        self.assertEqual(prototype.packet_fresh(self.packet, changed, [])[0], False)

    def test_late_decision_refuses(self):
        self.assertEqual(prototype.packet_fresh(self.packet, self.revisions, [{"affects_task": True}])[0], False)

    def test_transition_carries_basis_and_stops_on_stale_card(self):
        request = prototype.transition_request("card-1", "Review", "rev-1", "rev-1", ["Review"])
        self.assertEqual(request["basis"], "rev-1")
        with self.assertRaises(prototype.Refused):
            prototype.transition_request("card-1", "Review", "rev-1", "rev-2", ["Review"])

    def test_non_operator_cannot_leave_acceptance(self):
        with self.assertRaises(prototype.Refused):
            prototype.transition_request("card-1", "Done", "rev-1", "rev-1", ["Done"])


class EvidenceTests(unittest.TestCase):
    def test_changed_output_invalidates_receipt(self):
        with tempfile.TemporaryDirectory() as raw:
            workspace = Path(raw)
            (workspace / "one.txt").write_text("one")
            config = {"repository_revision": "abc"}
            receipt = prototype.evidence_receipt(workspace, b"task", b"accept", ["check"], config, 0, b"ok")
            self.assertTrue(prototype.evidence_matches(receipt, dict(receipt)))
            changed = dict(receipt, output_sha256=prototype.digest_bytes(b"different"))
            self.assertFalse(prototype.evidence_matches(receipt, changed))

    def test_empty_check_result_remains_unknown(self):
        self.assertNotEqual([], ["pass"])


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
