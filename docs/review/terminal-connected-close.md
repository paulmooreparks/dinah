# Connected-client terminal close validation

The old close test called Close after Toolhelp exposed ping's PID. A process can be visible before connecting to its console, while ClosePseudoConsole documents its close event for connected clients. Quality independently reviewed the standalone Win32 reproduction and confirmed this precondition gap in message 3694. The lead ratified an output-based precondition in 3696 and the OSC correction in 3698; Quality added the first-reply requirement in 3699.

The test now records the output offset at the command write and waits for three occurrences of the address after that point, covering cmd's echo, ping's header and the first reply without depending on localised words. It removes escape sequences first because cmd also puts the command in an OSC title. The parser starts at the beginning of the captured stream so a sequence split at the write offset cannot leak its payload into the count. The original close assertion, detached-child assertion and post-close time limit remain. A timeout includes the visible transcript since the write.

The runtime retains the separately approved input ordering. Input remains open until ClosePseudoConsole returns, while the output pump continues independently. A deterministic guard holds the native close pending and checks that input is retained until return. This ordering is a contract correction and is not claimed to solve a connecting-client race. On pre-24H2 Windows, a never-returning native close retains its input pipe and teardown goroutine until process exit; this limit is documented.

## Matched comparison

Both binaries were built from the same patch. The PID-only control omits only the new readiness wait; both contain the same native ordering, output capture, failure cleanup and approved PowerShell-specific handling. Each batch ran two full terminal-package processes with fifteen iterations each, alongside a seventy-five-iteration Ctrl-C process. The driver and all descendants were pinned to two CPUs and held the shared test lock. No console-membership helper ran in this comparison.

| Batch | Close cases | Close failures | Detached-child failures | Other failures |
| --- | ---: | ---: | ---: | ---: |
| PID-only control | 30 | 3 | 0 | 0 |
| Connected-output precondition | 30 | 0 | 0 | 0 |

Both separate seventy-five-iteration Ctrl-C loads passed. The corrected full-package runs passed with no skips. This does not resolve the separately recorded CI-only pre-output Node stall; that finding remains open.

## Guard evidence

The focused pty and terminals run passed 36 tests including subtests, with no failures or skips. The new TestPingReadinessNeedsVisibleOutput covers the retained failing transcript, header-only output, a first reply, output offsets, BEL and ST, non-title OSC and other control strings. A temporary negative control bypassed escape filtering and failed the header-only, OSC, control-string and partial-sequence cases. The original source was restored before the candidate binary was built.

The known-limit sentence approved by the lead is in DESIGN.md's Terminal section, invariants.md's known limits, and terminal-shells.md. The scoped implementation is 313b6079531957c28e16600fc170de3623e37a5a on fix/terminal-connected-close, based on mapped master 80b56e395b9390e576b6a99ddde38b863f372d96. This record is a documentation-only child of that implementation. Investigation-only native bindings and standalone research artifacts are kept on the separate research branch.

The retained evidence is under the temporary engine-term-20261102 folder: connected-focused.jsonl, connected-no-filter-control.jsonl, pid-baseline-30 and connected-30, with their drivers and exact compiled fixtures. The final profile check is recorded separately as connected-final-profile.jsonl. No install, restart, desktop operation or real-provider action occurred.
