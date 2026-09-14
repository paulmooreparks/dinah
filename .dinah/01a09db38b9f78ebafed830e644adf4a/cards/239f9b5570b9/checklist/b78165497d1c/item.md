---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:17:52Z
ordinal: 22
note: "dinah-456 section 4.3 rules that the workstream listing prints `workstream/<slug>`. Making that ruling true without the second half would break the reader it protects: `dinah workstream get workstream/addressing` and `dinah join pb-1 workstream/addressing` are both refused at 22a35fc, while `dinah contents addressing` is refused too, so each spelling works with one half of the surface and neither works with both. WorkstreamByRef is the single resolver every workstream-taking path reaches, so the tolerance goes there and the two callers that strip the prefix themselves pass the whole reference instead. Its call sites are produced by `grep -rn 'WorkstreamByRef(' --include=*.go . | grep -v _test.go`, which at 22a35fc prints seven callers besides the declaration: bench/entity.go:378 (resolveWorkstreamRef), bench/resolve.go:150 (ResolvePath), verb/beyond.go:962 and :1115, verb/mutate.go:610 and :640, and verb/query.go:551 for `query workstream=`. No workstream-taking path avoids it, so one TrimPrefix at its head reaches all of them. A doubled prefix stays refused, because no surface prints one. This survives dinah-456 OQ-1 either way, since the generic pair would resolve through ResolvePath and so through the same function. It also opens no compatibility risk: no recorded event carries a workstream reference, so no fixture under internal/bench/testdata/compat/ moves."
---
Workstream.Ref() returns the prefixed spelling, and Bench.WorkstreamByRef accepts both spellings with exactly one strip.