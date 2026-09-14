---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:24Z
ordinal: 17
note: "Three rules resolve a key: a string literal, an identifier bound to a module-level string const in the same file, and a template whose head matches a declared family. Anything else is a failure naming the file and line. A sweep that skipped what it could not read would report success over the call sites it happened to understand, and a reader could not tell that from a sweep that checked everything. Measured: 102 key expressions at 65a80ad, 0 unresolved."
---
A key expression the sweep cannot resolve fails the test rather than being skipped.