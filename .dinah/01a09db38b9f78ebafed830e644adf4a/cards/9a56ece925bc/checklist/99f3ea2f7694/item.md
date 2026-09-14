---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:18Z
ordinal: 14
note: Changing `ResolvePath`'s workstream arm would fix `edit` in one line and would also move `dinah path workstream/<slug>`, whose directory answer dinah-456 settled deliberately and whose job is to hand a shell a filesystem address. That is a settled behaviour of a second command being moved to fix a first, so `edit` gets its own declared target instead. `ResolvePath` has exactly two other callers, `runPath` at cmd/dinah/commands.go:1227 and the library's path verb at internal/verb/read.go:863, so the blast radius of the alternative was checked rather than assumed. The references guide's table is silent on what `path` prints for a workstream, so nothing documented moves either way.
---
`ResolvePath` keeps answering a workstream's directory; the fix lives in a new `Bench.ResolveEditTarget`.