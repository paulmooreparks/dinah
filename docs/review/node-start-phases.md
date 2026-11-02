# Node startup failure diagnostics

The required Windows Ctrl-C test has repeatedly found a running Node process without its first output on CI. The terminal-close correction does not explain this separate finding. No runtime cause has been established.

The diagnostic keeps the inline HTTP server, its output marker, the assertions and the fifteen-second waits. A fixture-owned journal records entry into JavaScript, the listen callback, whether stdout reports a TTY, and return from the original console.log call. Each record includes the Node PID and wall-clock time. A timeout reports the journal alongside the captured terminal output and the existing process snapshot, now including kernel and user CPU time.

The journal contains at most four records per attempt. It contains no environment dump or process report. Its errors are caught so diagnostic I/O cannot replace the server's behavior with an exception. A missing record therefore does not prove that its phase was never reached; a journal failure remains an alternative explanation. The actual journal PID identifies the fixture, while the process snapshot can include unrelated Node processes created during the same interval.

The synchronous journal writes and longer typed command can change scheduling and shell input behavior. A clean run with this instrumentation cannot establish that the startup stall is fixed. If CI reproduces it, the last phase narrows the next investigation. A returned console.log call does not prove that its asynchronous output reached Dinah or was displayed.

[Node's documented process I/O contract](https://nodejs.org/download/release/v24.4.1/docs/api/process.html#a-note-on-process-io) states that Windows TTY output is asynchronous and Windows pipe output is synchronous. The diagnostic records isTTY rather than assuming which route the pseudoconsole exposes. It does not use that observation as a runtime branch.

This branch is diagnostic only and is based on the independently accepted terminal-close correction c1827da. No assertion, timeout or retry policy has changed.

After Release's explicit slot handoff in message 3722, the full terminals package passed all 27 tests with no failures or skips. A focused rerun after adding successful-run journal reporting passed and recorded all four phases for the same PID, with stdout reporting a TTY. The artifacts are node-phases-focused.jsonl and node-phases-journal.jsonl under the temporary engine-term-20261102 evidence directory. The startup stall did not reproduce. These checks validate the diagnostic's successful path only.
