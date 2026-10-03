import importlib.util
import sys
from pathlib import Path

root = Path(sys.argv[1])
spec = importlib.util.spec_from_file_location("candidate", root / "readiness.py")
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)
cards = [
    {"id": "done", "state": "done", "depends_on": []},
    {"id": "a", "state": "ready", "depends_on": ["done"]},
    {"id": "b", "state": "working", "depends_on": []},
    {"id": "c", "state": "ready", "depends_on": ["missing"]},
    {"id": "d", "state": "ready", "depends_on": []},
]
assert module.ready(cards) == ["a", "d"]
assert module.ready(iter(cards)) == ["a", "d"]
try:
    module.ready([{"id": "a", "state": "ready", "depends_on": []}, {"id": "a", "state": "done", "depends_on": []}])
except ValueError:
    pass
else:
    raise AssertionError("duplicate identifier was accepted")
try:
    module.ready([{"id": "a", "state": "ready", "depends_on": ["b"]}, {"id": "b", "state": "ready", "depends_on": ["a"]}])
except ValueError:
    pass
else:
    raise AssertionError("dependency cycle was accepted")
print("4 acceptance groups passed")
