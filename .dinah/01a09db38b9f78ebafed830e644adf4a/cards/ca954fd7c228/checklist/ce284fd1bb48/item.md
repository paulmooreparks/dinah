---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:25Z
ordinal: 20
note: "VS Code fills nothing into a `package.nls` value, so one written there would render as its own characters. A parity check over that namespace would compare 273 pairs carrying 0 placeholders and assert nothing, which is the vacuous half this card warns about. Measured at 65a80ad: 312 values across eight files, 0 containing a brace."
---
The manifest namespace gets an assertion that it carries no placeholder at all, rather than a placeholder-parity check.