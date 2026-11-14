# Ollama context verification

The change is on `engine/ollama-context` in `.worktrees/engine-ollama-context`, from master `24ae30b8`. Source commit `4b6e046bada19bf71a1188bee12b781bd3e49251` adds the model-based local context, the Settings ceiling and the saved exhaustion warning. The contract and documented API references are in [Ollama context selection](../design/ollama-context-cap.md).

Previously, an automatic local chat sent no `num_ctx`, even though the adapter already read the model's maximum through `/api/show`. This inherited the server's smaller default. Automatic local requests now specify the model's maximum up to a 65,536-token default ceiling. Settings under Agents, Ollama can change that ceiling. Smaller chat choices remain effective. Cloud allocation is unchanged because the documentation does not promise that a local `num_ctx` can cap it.

## Automated checks

The final Ollama adapter package passed in 27.879 seconds through the shared FIFO. It covers the model and Settings ceilings, explicit choices, missing and invalid metadata, changed Settings reaching an open chat, full-window stops, completed tool-call/result pairs, and a length stop below the window that does not claim context exhaustion. The earlier package run passed in 26.661 seconds. Two old assertions were updated because automatic local context is now explicitly sent and known.

Focused desktop checks passed in 0.535 seconds, including Settings persistence and reset, rejected values, the page method inventory and saved warning rows. The two browser suites, `ollama-context-cap` and `ollama-round-limit`, passed together in four seconds. The first browser launch could not find the shared Playwright module; setting `NODE_PATH` to the existing root dependencies resolved that setup error without installing anything.

## Hidden local model

The executable harness is `scripts/ollama-context-live.cjs`. It uses an isolated home, an isolated Ollama server with cloud disabled, and a copy of the installed `qwen3.5:4b` manifest and its four blobs. No model was downloaded. Environment variables are copied from an explicit operating-system whitelist, and no account configuration or credential file is copied. The HTTP observer forwards requests and streamed responses unchanged. Only headless Edge sees the hidden window.

Ollama 0.34.1 reports architecture `qwen35`, maximum context 262,144 tokens and model digest `2a654d98e6fba55d452b7043684e9b57a947e393bbffa62485a7aac05ee4eefd`. The server was deliberately configured with an 8,192-token default, making accidental inheritance distinguishable from the new behaviour.

| Observation | First hidden run | Committed build |
| --- | ---: | ---: |
| Automatic request `num_ctx` | 65,536 | 65,536 |
| Allocated context from `/api/ps` | 65,536 | 65,536 |
| Cap saved through the real Settings page | 8,192 | 8,192 |
| Subsequent task request `num_ctx` | 8,192 in every exchange | 8,192 in every exchange |
| Subsequent allocated context from `/api/ps` | 8,192 | 8,192 |
| Last prompt count plus generated count | 8,127 + 65 = 8,192 | 8,150 + 42 = 8,192 |
| Files created before exhaustion | 8 of 60 requested | 16 of 60 requested |
| Exchanges including the initial short response | 17 | 34 |
| Page errors | 0 | 0 |

The first run's last response requested a read of the eighth file and reported `done_reason: length`. Dinah completed and saved that read's result, then stopped without another model exchange. The chat visibly said that its context window was full and the task might be unfinished. The saved transcript contains that warning as a status row followed by `Turn ended: max_tokens`. The finding depends on the counts reaching the requested window, not on interpreting that reason string.

The first run's evidence is under `%TEMP%\ollama-context\live-1`. Its observation log has SHA-256 `ec518d05d6533c27932378f778a4b95b1f434e5947742a2d5c501f0e432019cc`, and its saved transcript has SHA-256 `33a50e61ea2cb780c7aa390695275cbd18a90dd4b19b37def4229bfd285c66da`. The first binary preceded the final record revision marker and below-window reason wording. The final run used the committed source with no product edits. Its last response contained text without a tool call, and its counts also reached 8,192. The task remained unfinished. The warning was visible and saved, and no further model exchange started.

The final binary, built with `desktop,production`, has SHA-256 `a48c15fbd188a994c6a862f40c4dc06164d507ece02e0c9ade01f8de48b57e14`. The five copied model files total 3,389,984,444 bytes. Ollama also wrote metadata inside the isolated store. These are observations of one installed local model, not a guarantee that a model completes a task, or a claim about cloud models. No push, merge, installation, visible interaction or write to the real application homes occurred.

The committed-build evidence is under `%TEMP%\ollama-context\live-final`. Its observation log has SHA-256 `6d3cc2a72599fcee5067c2e2e20c60be9f6e690aef95f253f93491f96d0280a3`; the saved transcript has SHA-256 `5602bde6cbe6e0b703d577d6d0ad8a614781437c4728453c6b1b457b62a57c3c`. The visible warning and saved status row were checked in both runs. Reload and process restart persistence were not separately exercised here; the existing saved-status test passed.

Both runs stopped their owned window and Ollama process trees. A process inventory afterwards found no Ollama process or executable running from the trial folder. Every copied model file had one hard link and no symbolic link when checked. Automatic approval review refused the normal, path-checked recursive removal with only “blocked by policy”, so no alternative deletion was attempted. The model copy remains at `C:\Users\paul\AppData\Local\Temp\ollama-context\models` for the lead to remove. The two run folders retain the invented workspace, transcript, screenshot and logs as evidence; the build executables remain in the trial root. Nothing in the original model store was changed.

Please review the source and evidence, then integrate if accepted. The only cleanup requested is removal of the isolated model copy and expendable build executables after review.
