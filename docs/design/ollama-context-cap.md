# Ollama context selection

Local Ollama chats now send `options.num_ctx` on every model exchange. Automatic context uses the model's `<architecture>.context_length` from `POST /api/show`, bounded by the Ollama Context Limit in Settings. A smaller per-chat choice is preserved. The Settings ceiling does not rewrite a larger saved choice; the existing model-change rule still clamps choices beyond the model's own limit. Settings changes apply to the next exchange. The context meter and full-window controls use that same effective size.

The default ceiling is 65,536 tokens. Ollama recommends at least 64,000 tokens for agents and coding, while warning that larger windows require more memory. The person can lower or raise the ceiling in Settings under Agents, Ollama. Resetting it restores the default. If the model reports no usable context metadata, automatic mode explicitly requests 4,096 tokens, still bounded by Settings, and the chat's context option explains the fallback. An explicit chat choice remains possible.

Cloud models keep the existing behaviour. Ollama documents their default as the model's maximum, but does not document that a local `num_ctx` controls a remote model's allocation. This change therefore promises the configurable ceiling only for local models. No account-backed model is used for verification.

After a response, the documented `prompt_eval_count` and `eval_count` are added. If they reach the context size requested for that exchange, the chat receives a saved warning that the window is full and the task may be unfinished. Completed tool calls receive their results before the turn stops. No further exchange is started. A `done_reason` of `length` below that size retains the existing stop mapping and now displays the reported reason, without attributing it to context exhaustion. Ollama's reference does not enumerate that field's values, so the context-full finding depends on the token counts, not that string.

The settings file gains one optional integer, `ollamaContextCap`, with zero or absence selecting the default. Older builds ignore it. Conversation records keep their existing format and explicit choices. Their zero value now selects the model and Settings ceiling instead of inheriting the server's default.

The implementation uses Ollama's [show response](https://docs.ollama.com/api-reference/show-model-details), [chat options and token counts](https://docs.ollama.com/api/chat), [num_ctx parameter](https://docs.ollama.com/modelfile), and [context guidance](https://docs.ollama.com/context-length).
