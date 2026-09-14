---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:06Z
ordinal: 13
note: An attachment mounts nothing, so a MountOf guard written without care refuses `dinah attach <attachment> <file> --replace`, which is a legal act that rewrites an attachment's bytes and writes nothing below it. It exits 0 at 22a35fc and must go on doing so. `internal/verb/beyond.go:190` already carries the condition `req.Replace && entity.Kind == "attachment"` inline; the fix binds it to a local named `replacing` and both the guard and the branch read that local, so a later edit cannot move one without the other. AC-2 pins the replace case.
---
A legal `attach --replace` against an attachment is not refused, and one expression decides both the refusal and the write branch.