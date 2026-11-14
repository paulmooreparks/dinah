# Ollama memory verification

The change is on `engine/ollama-memory` in `.worktrees/engine-ollama-memory`, from master `80af29ce`. Source commit `21bc583fd7f4234f9d7882b2260ab2e377e2409a` changes the automatic default to 32,768 tokens and adds a read-only loaded-memory display beside the ceiling in Settings. The [design note](../design/ollama-context-cap.md) contains the measurements, API contract and default-selection reasoning.

The installed local `qwen3.5:4b` reported 3.11 GiB at 8K, 3.91 GiB at 32K and 4.97 GiB at 64K through Ollama's documented ps API. All of that reported allocation was in GPU memory. The machine has 127.94 GiB of physical RAM and 8 GiB of dedicated GPU memory. The chosen budget is 5% of RAM and 50% of GPU memory for one local model. The largest tested context inside both targets is 32K, which saves 1.06 GiB compared with 64K. This is a default choice, not a runtime memory limiter or a guarantee for every model.

First-token times including loading were 84.921, 85.948 and 83.498 seconds at 8K, 32K and 64K. Loaded medians from three further responses were 84.0, 86.7 and 71.7 milliseconds respectively. These short-prompt measurements do not establish long-prompt latency or peak operating-system memory use.

## Verification

The full desktop package passed in 166.428 seconds, including the page-file encoding test. The full Ollama adapter package passed in 25.769 seconds. A final focused adapter run passed in 0.339 seconds after making the live-cap test change from the new default to a different size. No production source changed after the full package run.

The browser suites `ollama-memory`, `ollama-context-cap` and `ollama-round-limit` passed together in four seconds. They cover reported total and GPU memory, other-context readings, no loaded model, an unavailable server, fixed result dimensions and refusal of a late result for an older ceiling. The Settings file is valid UTF-8, with no replacement characters or mojibake markers.

The real hidden window was built with `desktop,production` from the same source. Before a chat sent a prompt, Settings displayed no loaded model and the observer counted zero chat requests. The chat then sent `num_ctx:32768`. The App query, ps API and visible Settings text agreed on 4,197,596,527 bytes, displayed as 3.91 GiB, for total and GPU allocation at 32,768 tokens. The result area retained its width and height. There were no page errors. The screenshot and text capture are in the live evidence folder. This check did not exercise a page reload or a program restart.

All model calls used a private, cloud-disabled Ollama 0.34.1 server, an isolated home and a copy of the installed model's manifest and four referenced blobs. Those five copied files total 3,389,984,444 bytes; Ollama also wrote metadata in the copy. Nothing was downloaded, and no account-backed provider, credential file or token file was used. The original application homes and model store were unwritten.

## Evidence and cleanup

The scripts `scripts/ollama-memory-trial.cjs` and `scripts/ollama-memory-live.cjs` reproduce the measurements and hidden window journey through the shared FIFO. Raw logs are under `C:\Users\paul\AppData\Local\Temp\ollama-memory`. Their SHA-256 values are recorded here.

| Artifact | SHA-256 |
| --- | --- |
| `8k/observations.jsonl` | `971c3d9d48c6daa4d0456da3803072379a72fcbb3b7db33d94f7f2a20365e936` |
| `32k/observations.jsonl` | `f41ee98e8eaada231e38cf24886b449f1e81e9e6848953a23853aa5608db54da` |
| `64k/observations.jsonl` | `952e85be49539cf985d4ec9fb94df6684516a0d4d75d05045d3428ae97f1af8c` |
| `live/observations.jsonl` | `0145a3fd36182f0c37e918fcf885b102800370128c7a2de6cb3046dd01f06249` |
| `desktop.exe` | `fd7c14960c97ed48476d99f9c0a2f2f7f8863581bbe6621f54264e71c0eadfd3` |

All owned process trees have stopped. A process inventory found no Ollama process or executable running from the trial folder. Every copied model file was checked to have one hard link and no symbolic link. Automatic approval review rejected the normal, path-checked recursive deletion with only "blocked by policy". No alternative deletion was attempted. The model copy remains at `C:\Users\paul\AppData\Local\Temp\ollama-memory\models` for the lead to remove; the expendable build program is `desktop.exe` in its parent folder. The four run folders retain invented evidence.

Please review for integration and remove the copied models and expendable build program after review. No push, merge, installation or visible interaction occurred.
