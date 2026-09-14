---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:15Z
ordinal: 20
note: "Emptying unwrittenEvents makes gofmt write map[string]string{} on one line, and the script's pattern requires a newline before the closing brace, so the script would raise SystemExit and report nothing about the document. Leaving a dead exemption entry behind to keep the pattern matching is debt inside a guard, so the pattern is widened instead. The stray identifier is the second edit: the script's last check refuses a bare card identifier in published text and format.md line 20 carries one, which is why the script exits 1 at trunk 9260a2a and why a criterion asserting a clean exit would otherwise fail against a defect this card did not cause. Both were found by running the script at 9260a2a rather than by reading it. AC-4 is the check on both."
---
Two edits outside the obvious surface are in scope: the empty-map pattern in scripts/derive_event_counts.py, and the bare card identifier at docs/design/format.md line 20.