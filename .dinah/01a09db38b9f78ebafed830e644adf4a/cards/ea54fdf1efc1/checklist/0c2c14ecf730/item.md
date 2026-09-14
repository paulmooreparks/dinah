---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:11Z
ordinal: 24
note: The declaration looks like the right home for the card's own promise that what is true at a terminal is true over MCP, and it is not usable here. `crossHeadCases` at cmd/dinah/cross_head_test.go:165 drives a declared command from a `map[string]string` of named values that the terminal side renders as flags, and both of these commands take their reference and their field as positionals, so the fixture cannot express an invocation to compare and a declared entry would be compared on the no-argument call, which refuses on both heads and proves nothing. AC-1 runs every field of every kind through both heads and compares the values, which is a wider comparison than the declaration buys for the three commands that carry it. Widening the fixture to express positionals is a card of its own and this spec does not file one.
---
Neither `get` nor `set` joins `crossHeadIdentical`, and the two heads are compared directly by AC-1 instead.