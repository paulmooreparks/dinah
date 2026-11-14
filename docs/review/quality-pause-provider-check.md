# Independent pause-provider check

The test assertions were written from Engine's design note `docs/design/pause-provider-input.md` before reading its candidate implementation. The baseline is master bd1a2971. The independent files are `cmd/dinah-desktop/quality_pause_provider_test.go` and `cmd/dinah-desktop/testdata/quality-pause-provider.cjs`.

The cases exercise Codex turn steering, Claude's advertised extension before hooks bind, Devin's PostToolUse additionalContext, and the host-owned Ollama loop. Each case holds a working turn across the pause request, repeats that request, ends the turn, loads the pause record into a fresh App, and replays recovery to require one carry-on. Plain ACP must receive neither concurrent input nor cancellation. A separate idle Claude case must receive no prompt and create no pause record.

These are hidden Go integration tests with local protocol stand-ins and invented temporary state. The real provider adapters and bus/recovery code run in-process; the desktop executable is not restarted. The frozen idle Claude guard originally used the established test connection; the final case uses the real advertised Claude adapter and a synchronous peer call to fence prior traffic. The peer scripts model compliance with the pause request, so this cannot establish real model compliance or long-tool delivery timing.

The tests will be committed before integrating the candidate. Compiler or fixture corrections, if needed, will be recorded separately, without weakening the contract assertions.

## Candidate and result

The assertions were frozen in ebfe3946e7ea4335863fb5b31c23a668a521312a. Engine's candidate ee01fa5ca0f105352b39518a012ecd593475d7ab was then cherry-picked as a4adec6d on this test branch. No candidate production code was changed.

The final focused command passed all five provider cases and the separate idle Claude case in 7.567 seconds, exit zero, through FIFO ticket 1615:

```powershell
pwsh -NoProfile -File scripts/with-test-lock.ps1 go test ./cmd/dinah-desktop -run '^TestQualityPause' -count=1 -timeout=120s -v
```

Codex's wire request targeted the active turn. Claude requested promptRequired and received injected before ending its turn. Devin's first PostToolUse supplied one pause in additionalContext and its second supplied none. Ollama waited for the outstanding HTTP exchange before inserting the pause once. Plain ACP received no pause, concurrent prompt or cancellation; its record said it had not been told and it completed naturally. Each case then delivered one carry-on through the bus to a fresh adapter and App. Replaying the durable record left no duplicate waiting. Idle Claude advertised steering but received neither a prompt nor a steering request.

The first run failed because the harness demanded success from a repeated restart request, while the host rejects it as already waiting. The check now allows that rejection and still asserts one pause. Ollama also failed to reach the model because the test binary could not serve its managed applet MCP endpoint. The fixture now removes unrelated MCP server definitions from model-session creation and load; its exchange loop and steering remain real. A second run exposed the Codex peer's missing workspace field on resume, which is now preserved. The idle Claude check was strengthened to use the actual adapter, and cancellation notifications are recorded as violations as well as cancellation requests. No assertion on delivery, completion or deduplication was relaxed.

Node syntax and git diff checks passed. The suite used local subprocesses, a loopback model server and invented temporary state. No real home, credential files, visible windows, desktop input, real model, full desktop suite or browser suite was used. Nothing was pushed, merged into master or installed. The test branch includes Engine's cherry-picked candidate for execution; reviewers who already have that candidate need only the independent test and report commits.

This evidence supports the documented provider routing and recovery contract. It does not establish an operating-system process restart or real models obeying the safe-point instruction. The earlier process-restart checks remain separate evidence.
