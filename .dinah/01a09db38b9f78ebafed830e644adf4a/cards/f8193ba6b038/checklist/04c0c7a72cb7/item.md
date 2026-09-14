---
kind: decision
state: resolved
ts: 2026-09-14T02:16:31Z
ordinal: 16
note: CORE-JSON's interchange form already carries a profile member for the same reason. A version a caller can check before trusting the field order costs one short line and is worth having regardless of how OQ-1 (stability posture) is ruled, since even a format the operator declares frozen benefits from a caller being able to detect a future, deliberate break rather than silently misparsing it.
---
Every compact payload opens with a version record, fmt|compact|1, before any other record.