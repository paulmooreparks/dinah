import importlib.util
import sys
from pathlib import Path

root = Path(sys.argv[1])
spec = importlib.util.spec_from_file_location("candidate", root / "usage_ledger.py")
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)
ledger = module.UsageLedger()
identity = {"command": ["codex", "exec"], "prompt": "abc"}
usage = {"input_tokens": 100, "cached_input_tokens": 40, "output_tokens": 7}
ledger.import_run("r1", identity, usage, "complete")
ledger.import_run("r1", identity, usage, "complete")
assert len(ledger) == 1
try:
    ledger.import_run("r1", identity, {**usage, "output_tokens": 8}, "complete")
except Exception:
    pass
else:
    raise AssertionError("conflicting duplicate was accepted")
totals = ledger.totals()
def field(value, name):
    return value[name] if isinstance(value, dict) else getattr(value, name)
assert field(totals, "input_tokens") == 100
assert field(totals, "cached_input_tokens") == 40
assert field(totals, "output_tokens") == 7
ledger.import_run("r2", identity, None, "failed")
if hasattr(ledger, "get_run"):
    unknown_usage = ledger.get_run("r2")["usage"]
else:
    unknown = next(row for row in ledger.rows if (row[0] if isinstance(row, tuple) else row.run_id) == "r2")
    unknown_usage = unknown[2] if isinstance(unknown, tuple) else unknown.usage
assert unknown_usage is None
print("8 acceptance assertions passed")
