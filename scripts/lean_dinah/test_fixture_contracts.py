import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


FIXTURES = Path(__file__).parent / "fixtures"


USAGE_REFERENCE = '''
class UsageLedger:
    def __init__(self): self.data = {}
    def import_run(self, run_id, identity, usage, state):
        value = (run_id, identity, usage, state)
        if run_id in self.data and self.data[run_id] != value: raise ValueError("conflict")
        self.data[run_id] = value
    @property
    def rows(self): return list(self.data.values())
    def __len__(self): return len(self.data)
    def totals(self):
        known = [row[2] for row in self.rows if row[2] is not None]
        return {name: sum(item[name] for item in known) for name in ("input_tokens", "cached_input_tokens", "output_tokens")}
'''

USAGE_BROKEN = '''
class UsageLedger:
    def __init__(self): self.rows = []
    def import_run(self, run_id, identity, usage, state): self.rows.append((run_id, identity, usage, state))
    def __len__(self): return len(self.rows)
    def totals(self): return {"input_tokens": 0, "cached_input_tokens": 0, "output_tokens": 0}
'''

READINESS_REFERENCE = '''
def ready(cards):
    cards = list(cards)
    by_id = {}
    for card in cards:
        if card["id"] in by_id: raise ValueError("duplicate")
        by_id[card["id"]] = card
    visiting, visited = set(), set()
    def visit(card_id):
        if card_id in visiting: raise ValueError("cycle")
        if card_id in visited: return
        visiting.add(card_id)
        for dependency in by_id[card_id]["depends_on"]:
            if dependency in by_id: visit(dependency)
        visiting.remove(card_id); visited.add(card_id)
    for card_id in by_id: visit(card_id)
    return [card["id"] for card in cards if card["state"] == "ready" and all(dependency in by_id and by_id[dependency]["state"] == "done" for dependency in card["depends_on"])]
'''

READINESS_BROKEN = '''
def ready(cards):
    return [card["id"] for card in cards if card["state"] == "ready"]
'''


class FixtureContractTests(unittest.TestCase):
    def run_candidate(self, fixture, filename, source):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            (root / filename).write_text(source, encoding="utf-8")
            return subprocess.run([sys.executable, str(FIXTURES / fixture / "acceptance.py"), str(root)], capture_output=True, text=True)

    def test_usage_evaluator_accepts_reference_and_refuses_broken_candidate(self):
        self.assertEqual(self.run_candidate("usage_ledger", "usage_ledger.py", USAGE_REFERENCE).returncode, 0)
        self.assertNotEqual(self.run_candidate("usage_ledger", "usage_ledger.py", USAGE_BROKEN).returncode, 0)

    def test_readiness_evaluator_accepts_reference_and_refuses_broken_candidate(self):
        self.assertEqual(self.run_candidate("dependency_ready", "readiness.py", READINESS_REFERENCE).returncode, 0)
        self.assertNotEqual(self.run_candidate("dependency_ready", "readiness.py", READINESS_BROKEN).returncode, 0)

    def test_evaluator_interfaces_are_explicit_in_contracts(self):
        usage = (FIXTURES / "usage_ledger" / "contract.md").read_text(encoding="utf-8")
        readiness = (FIXTURES / "dependency_ready" / "contract.md").read_text(encoding="utf-8")
        for text in ("UsageLedger", "import_run(run_id, identity, usage, state)", "input_tokens", "cached_input_tokens", "output_tokens", "rows"):
            self.assertIn(text, usage)
        for text in ("ready(cards)", "one-shot generator", "Return a list", "ValueError", "missing dependency"):
            self.assertIn(text, readiness)


if __name__ == "__main__":
    unittest.main()
