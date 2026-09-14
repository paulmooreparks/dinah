---
kind: acceptance_criterion
state: verified
column: aa6cd1c6ae5f
ts: 2026-09-14T02:16:48Z
ordinal: 10
note: PASS, and the ordering it exists to prove was discriminated rather than assumed. A workbench outside the root had its anchor overwritten with garbage; opening it directly refuses `malformed`, so opening really would fail. Against the bound server, naming it answered outcome refused, refusal dinah.outside-root, with the root in context and no `malformed` anywhere. The verifier then armed the probe by running bench.Open ahead of the containment check in resolveLibrary, and the same call answered `malformed` with detail `title`, which proves the criterion distinguishes the two orders rather than passing under either.
---
Against that same server, a `tools/call` naming a workbench that lies outside `<dir>` and whose `workbench.md` has been made unreadable answers with `outcome` refused and `refusal` `dinah.outside-root`, carries the root in its `context`, and does not answer `malformed` or any other refusal that only an opened workbench can raise.