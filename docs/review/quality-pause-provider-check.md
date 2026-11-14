# Independent pause-provider check

The test assertions were written from Engine's design note `docs/design/pause-provider-input.md` before reading its candidate implementation. The baseline is master bd1a2971. The independent files are `cmd/dinah-desktop/quality_pause_provider_test.go` and `cmd/dinah-desktop/testdata/quality-pause-provider.cjs`.

The cases exercise Codex turn steering, Claude's advertised extension before hooks bind, Devin's PostToolUse additionalContext, and the host-owned Ollama loop. Each case holds a working turn across the pause request, repeats that request, ends the turn, loads the pause record into a fresh App, and replays recovery to require one carry-on. Plain ACP must receive neither concurrent input nor cancellation. A separate idle Claude case must receive no prompt and create no pause record.

These are hidden Go integration tests with local protocol stand-ins and invented temporary state. The real provider adapters and bus/recovery code run in-process; the desktop executable is not restarted. The idle Claude guard uses the established test connection. The peer scripts model compliance with the pause request, so this cannot establish real model compliance or long-tool delivery timing.

The tests will be committed before integrating the candidate. Compiler or fixture corrections, if needed, will be recorded separately, without weakening the contract assertions.
