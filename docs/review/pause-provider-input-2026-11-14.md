# Pause provider verification

The branch `engine/pause-unsteerable` starts from master `9758260a`. The design inventory is [pause-provider-input.md](../design/pause-provider-input.md), with links to the producer-owned contracts read on 14 November 2026.

Codex already supported the pause request through its active-turn steering adapter. A new hidden subprocess stand-in speaks the app-server protocol to the actual adapter. It keeps a turn running until it receives the full pause instruction through `turn/steer`, verifies the thread and expected turn IDs, and then completes the turn. The desktop test confirms that Pause Chats and Restart reaches that path and records the chat as told. The stand-in exits with an error if it receives an interrupt instead.

Claude gains its advertised `_session/steering` path when hooks have not reached the chat. Another protocol stand-in keeps the original ACP prompt open, checks the text and explicit host-owned idle policy, acknowledges injection and completes that same prompt. The desktop test verifies the capability and durable pause record without a hook binding. It treats cancellation as a failure. Adapter tests cover accepted input, an ended turn, unexpected outcomes, RPC failure, missing, false and malformed advertisements, and local rejection of image input. Session ID prefixes are checked on the wire. Unsupported or uncertain outcomes do not acknowledge delivery.

The shared capability check also retains reached-hook support when native steering is absent. Its regression checks that text hooks remain usable, image input stays unsupported and unreached hooks remain unavailable. Plain ACP agents retain the waiting path. Existing hook and Ollama protocol tests still exercise their supported delivery boundaries.

All commands ran through `scripts/go-gate.ps1`, which uses the shared FIFO. The package-wide run passed for `internal/claudeagent` (1.025 seconds), `internal/codexagent` (4.491 seconds), `internal/sessionhost` (24.177 seconds), `internal/hooks` (7.826 seconds) and `internal/ollamaagent` (43.444 seconds). After the final initialization reset adjustment, the complete Claude package passed again in 0.795 seconds.

The focused desktop run passed in 7.074 seconds. It included the new protocol tests, existing pause, cancellation and fresh-App resume flows, same-program continuation wording, reached-Claude-hook delivery and busy bus delivery cases. Its command was:

```powershell
pwsh -NoProfile -File scripts/go-gate.ps1 ./internal/claudeagent/... ./cmd/dinah-desktop -run 'TestAdvertisedSteering|TestPause|TestAChatIsToldWhatHappened|TestSameProgramResumeText|TestSteeringAClaudeChat|TestBusyBusDeliveryUsesSteeringCapability' -count=1
```

Formatting and `git diff --check` passed. No full desktop Go run, browser suite, real model run or native restart was performed for this branch. The tests establish documented protocol delivery and host bookkeeping; they do not prove that a real model will stop at a safe point. No live chat was asked to pause. No visible interaction, real-home write, credential read, push, merge or installation occurred. The branch is ready for the lead's review.
