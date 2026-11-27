# Hook observation lifetime repair

The Background chip could survive a restart with no live work because the host counted every durable unmatched subagent start for the native session. A missing stop had no age cutoff in this projection. The separate six-hour steering cutoff did not apply, and a resumable native session ID remained bound to the same chat across execution lifetimes.

## Reproduction and cause

The initial `TestUnmatchedSubagentStartIsNotLiveEvidenceAfterRestart` opened a real bus, bound a native session to a chat, recorded a subagent start, closed the database and reopened it. There was no live turn, connection, process or new hook. Its desired zero-count assertion failed with `count=1, err=<nil>`. No product code had changed when that result was reported.

The corrected regression now performs the host-authorized lifetime reset before the new native load. Merely opening the bus file is not a boundary because more than one client can use it. Native execution ownership belongs to a session on its connection, and the provider adapter is shared across chats.

## Implemented boundary

The host clears only the current chat's bound native hook observations before `session/load` or `session/resume`, after a confirmed `session/close`, and on connection loss. It also clears that chat's in-memory observed count. A refused close retains the live evidence. A failed reset blocks a fresh load rather than silently continuing with stale observations. No clock estimates worker completion.

The bus deletion checks both native-session identity and chat binding. An attempted reset for another chat leaves its records intact. Fresh starts arriving during the new load survive because the reset runs before the native call. Ordinary turn completion does not reset the count, so a live worker can continue to keep the chat busy until its stop arrives.

No delivery rule, native task cancellation, provider pin, page API or screen changed.

## Contract and remaining limit

The pinned [adapter contract](https://github.com/agentclientprotocol/claude-agent-acp/blob/v0.101.0/docs/air-extensions.md) describes owned session close and reopening, and permits the same historical session to be open in another process. The host does not infer a worker process from that historical ID.

The [Claude hook reference](https://code.claude.com/docs/en/hooks#subagentstop) describes SubagentStop for a subagent finishing its response. It does not establish that TaskStop or killing a worker emits the hook. **A TaskStop within the same live lifetime can still leave a stale chip when no SubagentStop arrives.** Resolving that case requires a documented complete task snapshot or another documented terminal signal; this change does not parse tool output or invent an ID mapping.

Before this repair, no existing product action reliably deleted the durable unmatched observation. CancelSubagent sends a cancellation request and cannot guarantee a matching hook. Restarting alone restored the row. The repair makes a subsequent native load or resume retire it. No live database was edited, no running chat was stopped and no action was taken on the reported chat during development.

## Verification

The focused bus and App tests passed. They cover durable restart cleanup, foreign-chat protection, reset before load, resume, confirmed close, refused close, connection loss followed by a real App reopen, and ordinary turn-end preservation. The fresh-start-during-load test is included in the full gate.

The full uncached Go gate passed every package except `internal/judge`. `TestALateJudgesCostIsCountedWhenItsRequestEnds` returned the correct late/rule result after 422.6 ms against a 400 ms assertion. The desktop package passed in 485.3 seconds. No judge source changed. The guide's single permitted retry of the unchanged judge package passed uncached under the shared lock in 19.7 seconds, exit zero. The first failed run is retained; neither its assertion nor its timeout changed, and the full gate was not repeated.

The four browser suites `background-tasks`, `background-work`, `chats-busy-count` and `subagents` passed in 19 seconds with exit zero and an empty failures file. The staged-file forbidden-terms guard passed verbose and uncached without a skip.

Validation logs are retained outside the repository under the system temporary folder as `dinah-hook-lifetime-full-go.log`, `dinah-hook-lifetime-judge-retry.log` and `dinah-hook-lifetime-browser.log`, with browser artifacts in `dinah-hook-lifetime-browser`.

Integration and installed-program verification belong to the integrator. This branch has performed no merge, push or install.
