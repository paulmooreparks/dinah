# Task

Implement `usage_ledger.py`. `UsageLedger.import_run(run_id, identity, usage, state)` must record a run once, accept an identical duplicate without adding another row, and reject a duplicate run identifier whose identity, usage, or state differs. Unknown or failed usage remains `None`; it must never become zero. Keep cached input as a subset category and do not add it to input when reporting totals. Use only the Python standard library.
