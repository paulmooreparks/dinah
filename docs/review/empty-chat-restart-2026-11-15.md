# Empty chats before their first turn

Quality checked master `038492f3` on `quality/empty-chat-restart`. The test homes and provider data were invented and temporary. No real provider, visible window or real home was used.

## Results

| Provider | Result on the pinned source | Evidence boundary |
| --- | --- | --- |
| Ollama | Fail before the fix; pass after the fix. | The real Ollama adapter creates an empty session against a loopback API stand-in, closes, starts again on the same isolated store, lists and loads the same ID without calling the model. |
| Codex | Pass. | The existing app-server protocol fixture requires the context-record persistence operation before a fresh connection can resume. Its no-first-turn case passes. |
| Claude | Pass at the host boundary. | A disk-backed ACP stand-in provides new, list and load. A fresh App recovers the ID and saved name without sending a prompt. This does not prove the vendor adapter saves an empty session. |
| Devin | Pass at the host boundary. | The same host check uses the Devin provider path and its unprefixed ID. This does not prove the vendor executable saves an empty session. |

The host check also passes for Ollama and Codex. It constructs a fresh App and provider connection on the same isolated home. The provider fixture deliberately stores its allocation; it cannot establish vendor-owned persistence. A native window restart and live vendor tests were not run for this check.

## Defect and correction

In `internal/ollamaagent/conn.go`, `open` constructed the new record at the original line 640 and published it in `c.sessions` without saving it. `session/list` reads the saved files, so a restart lost a chat that had not reached another operation that saves its record.

The new-session path now saves the settled record before publishing the session or returning its ID. A write failure refuses creation. Loading an existing session retains its previous behavior.

`TestEmptyConversationSurvivesRestartBeforeFirstTurn` failed on the unchanged source through FIFO ticket 1993 with an empty restored list. It passes after the correction. A second test uses a regular file where a directory is required and verifies that a failed save returns neither a session ID nor an in-memory session.

The first host fixture run expected the person's name in the provider title field. The UI receives the person's name separately from saved SessionTitles, so the fixture now asserts that persisted map instead. All four cases then passed. This was a fixture correction, not a product change.

## Verification

The focused Ollama and Codex cases passed through FIFO ticket 1996. The corrected four-provider host check passed separately. The complete touched Go suites passed: internal/ollamaagent in 65.812 seconds and cmd/dinah-desktop in 217.752 seconds, exit zero, through scripts/with-test-lock.ps1. The command was go test ./internal/ollamaagent ./cmd/dinah-desktop -count=1 -timeout=10m. Formatting and whitespace checks passed.

No push, merge or installation was performed. Quality retains ownership until the lead accepts the branch. Real Claude and Devin empty-session persistence remains unproved by these invented-provider checks.

