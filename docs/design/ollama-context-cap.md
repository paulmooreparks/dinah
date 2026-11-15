# Ollama context selection

Local Ollama chats now send `options.num_ctx` on every model exchange. Automatic context uses the model's `<architecture>.context_length` from `POST /api/show`, bounded by the Ollama Context Limit in Settings. A smaller per-chat choice is preserved. The Settings ceiling does not rewrite a larger saved choice; the existing model-change rule still clamps choices beyond the model's own limit. Settings changes apply to the next exchange. The context meter and full-window controls use that same effective size.

The default ceiling is 32,768 tokens, chosen from the measurements below. Ollama recommends at least 64,000 tokens for agents and coding, but that larger window used more than half this machine's dedicated GPU memory with the installed model. The person can lower or raise the ceiling in Settings under Agents, Ollama. Resetting it restores the default. If the model reports no usable context metadata, automatic mode explicitly requests 4,096 tokens, still bounded by Settings, and the chat's context option explains the fallback. An explicit chat choice remains possible.

Cloud models keep the existing behaviour. Ollama documents their default as the model's maximum, but does not document that a local `num_ctx` controls a remote model's allocation. This change therefore promises the configurable ceiling only for local models. No account-backed model is used for verification.

After a response, the documented `prompt_eval_count` and `eval_count` are added. If they reach the context size requested for that exchange, the chat receives a saved warning that the window is full and the task may be unfinished. Completed tool calls receive their results before the turn stops. No further exchange is started. A `done_reason` of `length` below that size retains the existing stop mapping and now displays the reported reason, without attributing it to context exhaustion. Ollama's reference does not enumerate that field's values, so the context-full finding depends on the token counts, not that string.

The settings file gains one optional integer, `ollamaContextCap`, with zero or absence selecting the default. An explicit saved ceiling, including 65,536, is unchanged by this default change. Older builds ignore it. Conversation records keep their existing format and explicit choices. Their zero value now selects the model and Settings ceiling instead of inheriting the server's default.

The implementation uses Ollama's [show response](https://docs.ollama.com/api-reference/show-model-details), [chat options and token counts](https://docs.ollama.com/api/chat), [num_ctx parameter](https://docs.ollama.com/modelfile), and [context guidance](https://docs.ollama.com/context-length).


## Memory measurement and the default

The installed `qwen3.5:4b` model was measured with Ollama 0.34.1 on 14 November 2026. Its show response reports architecture `qwen35` and a 262,144-token model maximum. Each context size used a fresh, isolated Ollama server and a copy of the installed model with cloud disabled. Nothing was downloaded. The machine has 137,377,075,200 bytes of physical RAM (127.94 GiB) and an NVIDIA GeForce RTX 3070 Ti with 8,192 MiB of dedicated GPU memory.

| Requested and reported context | Reported loaded size | Reported GPU size | Share of RAM | Share of GPU memory | First token after loading | Median first token while loaded (three trials) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 8,192 | 3.11 GiB | 3.11 GiB | 2.43% | 38.91% | 84.921 s | 84.0 ms |
| 32,768 | 3.91 GiB | 3.91 GiB | 3.06% | 48.87% | 85.948 s | 86.7 ms |
| 65,536 | 4.97 GiB | 4.97 GiB | 3.89% | 62.15% | 83.498 s | 71.7 ms |

The selection budget is at most 5% of physical RAM and 50% of dedicated GPU memory for one loaded local model, leaving room for the window, other programs and other agents. These are default-selection targets, not runtime memory limits. Of the three sizes tested, 32,768 is the largest within both targets. It saves 1.06 GiB of reported GPU allocation compared with 65,536, while the loaded first-token timings do not give a material reason to prefer the larger size. The person can still choose a larger ceiling explicitly. This model and machine do not establish a safe memory budget for every model or computer.

Each trial sent the same short one-sentence question, with only a trial number changed, `think:false`, `num_predict:32` and `temperature:0`. First-token time was measured from starting the HTTP request to receiving the first nonempty content chunk. The first request includes model loading and warm-up. Three further requests supplied the loaded median. These figures do not measure a long prompt's prefill time, operating-system peak working set or total GPU use by other programs. The allocation figures are Ollama's documented `size`, `size_vram` and `context_length` from [GET /api/ps](https://docs.ollama.com/api/ps), taken after every response. No extrapolation from file size or undocumented allocator behaviour is used.

The reproducible command is `node scripts/ollama-memory-trial.cjs <isolated-model-copy> <8192|32768|65536> <new-output-folder>`, through the shared test FIFO. The raw byte counts were 3,341,958,511, 4,197,596,527 and 5,338,447,215 respectively. Total and GPU byte counts were equal in these trials. Results and environment details are retained in the trial's JSONL logs.

## Settings memory reading

`OllamaMemory()` is a read-only page query returning `{limit, models:[{name,digest,size,sizeVRAM,context,targetContext}]}`. It reads only `/api/ps` and `/api/show`, with a two-second overall deadline. It does not load a model, resize one or send a prompt. `targetContext` is the automatic context at the saved ceiling, bounded by the model maximum; zero means that show failed and the target is unknown.

Settings places the reported loaded and GPU memory beside the context ceiling. A reading whose `context` equals `targetContext` is labelled as applying to that size. A reading at another context is labelled with both sizes and never presented as an estimate for the ceiling. If no model is loaded, Settings says that a figure will appear after a chat uses that size. Refresh reads again, and changing the saved ceiling refreshes the reading. The result area has fixed height with scrolling, and an older response cannot replace a result for a newer ceiling. Model names are rendered as text. Cloud models remain outside this local allocation display.
