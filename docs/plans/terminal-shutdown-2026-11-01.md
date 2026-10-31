# Terminal shutdown release investigation

Engine owns the investigation assigned in bus 2373. The stopped skins merge is bb2878939c70bf845f920383ba56799b6c315180. The comparison baseline is installed history merge c8c6038e7c6b0e4272afd2e3ff5e0f7633058223. Release remains held. The original timeout has not been explained. Quality's subsequent source-contract review establishes a separate completion defect, assessed below, so the initial no-correction disposition is withdrawn.

## Original evidence

Release's original receipts remain in `%TEMP%\dinah-release-skins-successor-20261101-003645`. Copies are preserved under `.worktrees/terminal-shutdown/.diagnostics`, including the reconstructed command receipt supplied by Release in bus 2377. The original JSON SHA256 is `71D5D9681709322EFE9843926541346599EA54499E41412B2F9F0E4D90AB63AD`.

The original invocation ran from `C:\Users\paul\source\repos\studio\.worktrees\release-history`:

```powershell
$env:DINAH_FORBIDDEN_TERMS = 'C:\Users\paul\.dinah\forbidden-terms.txt'
pwsh -NoProfile scripts/go-gate.ps1 ./... -count=1 -json
```

Release supplied no explicit package/test parallelism, timeout, GOMAXPROCS, GOFLAGS or CGO_ENABLED override. Ambient values were not captured at that time. Formatting and vet passed before the failed command. The original exit status was 1.

The assertion occurred about 15.072 seconds after the test began. It waits for `terminal:exit`, which the service emits only after both the output Read loop and shell Wait finish. Its text does not establish that the shell remained alive. Cleanup subsequently reported that the temporary directory was still in use. The original directory remains empty and present. No process survived in the original test's creation interval when Engine inventoried it, so that inventory cannot identify the original holder. No process was terminated during this investigation.

The baseline-to-candidate range changes none of `internal/terminals`, `internal/pty`, `internal/procjob`, `go.mod` or `go.sum`. That narrows source provenance but does not explain the failure.

## Controlled observations

The candidate worktree is `C:\Users\paul\source\repos\studio\.worktrees\terminal-shutdown`, on `fix/terminal-shutdown`. The baseline worktree is its sibling `terminal-baseline`, on `investigate/terminal-baseline`. Both started at the immutable revisions above. Go reports `go1.26.0 windows/amd64`; Windows reports build 26300.

All reproduction commands use the shared machine gate. No browser suite ran. No deadline or assertion was weakened. The first run used unchanged candidate source. Temporary test-only instrumentation then opened a separate shell process handle, recorded shell/child process IDs and creation times, observed Session.Wait independently, and captured process wait states and goroutine stacks at the original assertion deadline. The failing branch was never reached. The original and instrumented test sources are retained as `service_windows_test.original` and `service_windows_test.instrumented` in the diagnostics directory.

| Run | Command arguments after scripts/go-gate.ps1 | Observation |
| --- | --- | --- |
| Unchanged candidate terminal package | `./internal/terminals -count=20 -json -trace=.diagnostics/terminal-original.trace` | All twenty package repetitions passed. PowerShell split the unquoted trace argument, so no trace was produced. |
| Instrumented candidate focused test | `./internal/terminals '-run=^TestClosingATerminalEndsTheConsoleProgramsItsShellStarted$' '-count=200' -failfast -json '-trace=.diagnostics/terminal-instrumented.trace'` | All 200 passed, each in 0.05 to 0.10 seconds. The runtime trace and process identities are preserved. |
| Instrumented candidate backend and desktop | `./internal/... ./cmd/dinah-desktop '-count=1' -json` | All 48 tested packages passed. This command omitted the CLI package. |
| Unchanged candidate CLI | `./cmd/dinah '-count=1' -json` | The additional CLI package passed separately. This does not turn the preceding run into one full parallel gate. |
| Unchanged baseline | `./... '-count=1' -json` | All 49 tested packages passed. |
| Instrumented candidate, original explicit profile | `./... '-count=1' -json` | All 49 tested packages passed. The target test passed in 0.13 seconds. |

No explicit environment override was applied to the first five runs. A captured whitelist records GOMAXPROCS, GOFLAGS, CGO_ENABLED, DINAH_FORBIDDEN_TERMS and DINAH_BROWSER_TEST_TICKET as absent. No `-p`, `-parallel` or `-timeout` override was supplied. Ordinary Go defaults were retained. The baseline command ran from the baseline worktree; the other commands ran from the candidate worktree. Exact runnable diagnostic scripts and combined JSON/exit receipts are retained alongside the source snapshots.

The final candidate run explicitly set DINAH_FORBIDDEN_TERMS to `C:\Users\paul\.dinah\forbidden-terms.txt`, matching Release's recorded setting. It ran from 00:50:42 to 00:52:18 on 1 November, using the original command's complete package set. `run-exact-candidate.ps1` records that command and restores the original test source and environment in a finally block. The source instrumentation was the sole code difference during this reproduction; the worktree contains no retained product or test changes. This was an investigation run and supplies no release approval.

All six recorded commands exited zero. The temporary test instrumentation was removed after the final run. The shared test slot is released with the handoff. `terminal-shutdown-evidence.json` beside this document records SHA256 hashes for the original failure, successful run receipts, source snapshots, scripts and runtime trace; the artifacts themselves remain in the diagnostics directory.

## Windows contract and remaining uncertainty

Microsoft documents that ClosePseudoConsole sends CTRL_CLOSE_EVENT to connected clients. Starting with Windows 11 24H2 it returns immediately; output EOF establishes that all clients disconnected. The output pump continues draining on its own goroutine, consistent with that requirement. See [ClosePseudoConsole](https://learn.microsoft.com/en-us/windows/console/closepseudoconsole).

Microsoft also warns that closing while a client is still starting and connecting can produce an initialization error dialog. The test closes after observing child process creation, which does not independently establish completed startup. This remains a hypothesis because no reproduced failure or original process-lifetime evidence establishes it here. See [Creating a Pseudoconsole session](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session).

Passing runs establish successful lifecycles under their recorded conditions. They do not identify the original blocked stage or directory holder and do not establish a load explanation. Quality should review the evidence and this unresolved disposition before Team Lead decides the next release action. An isolated pass does not close the incident.

## Completion contract assessment after Quality review

Quality's bus 2379 finding is valid. The comment at `pty_windows.go:449` incorrectly equates ClosePseudoConsole return with drained output and released console/folder resources. Session.Close observes shell termination and the API call's return, but not output EOF. Service.CloseAll joins those Close calls and explicitly omits pump completion. The folder-release test assumes a stronger result. The host shutdown path in `cmd/dinah-desktop/app.go` also calls CloseAll before allowing the process to end or the update to start. On modern Windows that ordering can let the host exit before its clients finish disconnecting and its output pipe closes. No successful reproduction overrides the documented API contract.

The existing terminal:exit event already waits for both output Read termination and Session.Wait. Therefore adding an output-completion wait would address the separate shutdown guarantee; it does not explain why the original test failed to receive terminal:exit. The original incident remains unresolved.

### Callback and cleanup implications

The output reader invokes emit synchronously. If an output callback invokes a CloseAll that waits for pump completion, the pump cannot resume reading until its own callback returns. Waiting for a terminal:exit callback creates the same dependency. Neither the service mutex nor sync.Once solves that cycle. On older Windows even the existing synchronous CloseAll can wait for ClosePseudoConsole's bounded thirty-second wait while an output callback prevents draining. Session.Close must not acquire an unconditional output-completion wait that callback paths would inherit.

An output-read error also needs a distinct result from clean EOF. Closing the local pipe to unblock a reader cannot be reported as proof that console clients disconnected. A bounded shutdown that times out must report incomplete completion; returning success would preserve the defect under a different name.

The directory guarantee must cover resources owned by the terminal. A deliberately detached program may legitimately retain its inherited current directory after the console closes. The existing no-descendant folder test can require immediate removal after confirmed shutdown, but a general promise that no descendant holds the folder would contradict detached-program survival.

### Recommended correction contract

The correction should separate close requests from host completion waits. The service's individual and aggregate close requests should initiate closure and return without awaiting output or callbacks, so an emit callback can request closure safely. A separate host shutdown operation should reject new terminal starts, account for starts already admitted, request closure, and wait for shell termination and a clean output EOF for each admitted session. Completion must be signaled by the reader independently of emit callback return, before terminal:exit is delivered. Shutdown must never hold the service mutex while waiting or invoking emit.

The host shutdown operation belongs to the host lifecycle caller, outside emit callbacks. Its bounded context must return an error on incomplete shutdown, which the host must retain and report instead of claiming cleanup succeeded. Callback callers use the request operation and must not wait for the output they themselves are servicing. This is an explicit caller contract; goroutine identity or undocumented Windows behavior must not select between the operations.

Before implementation, Team Lead and Quality should confirm that contract, including the host's handling of an incomplete shutdown. Focused verification should establish that output EOF is required even when the console API returns early, that a callback can request aggregate closure without waiting on itself, that timeout/read errors remain incomplete, and that admitted concurrent starts cannot escape shutdown. Real Windows coverage should retain attached-program termination, detached-program survival and immediate folder removal for the no-descendant case. These are tests for the correction's own contract; they must not be presented as proof of the original timeout's cause.

This assessment required no new test run, process cleanup or product mutation. The shared gate remains free. Engine retains ownership of the source-contract issue pending the coordinated correction contract; the documentation-only result supplies no release approval.
