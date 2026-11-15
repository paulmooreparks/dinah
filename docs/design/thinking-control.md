# Thinking controls

The toolbar will expose the agent's select options in category `thought_level` beside Model. Each control uses Thinking as its label and the option names and values returned by the agent. A chat without such an option has no Thinking control. This follows the existing option path for pending choices, provider defaults, saved chat choices and substitutions. No effort value or provider-specific mapping is invented.

## Documented inventory

This inventory was checked on 15 November 2026. The upstream documentation describes capabilities; it does not prove the installed agent's catalogue. Dinah uses the actual catalogue for each chat.

| Program | Documented capability | Dinah boundary |
| --- | --- | --- |
| Codex | The [app-server model list](https://developers.openai.com/codex/app-server) reports supportedReasoningEfforts and defaultReasoningEffort. | Dinah's adapter already turns those values into a Thinking option in category thought_level. |
| Ollama | The [Thinking API guide](https://docs.ollama.com/capabilities/thinking) documents model-defined supported values and defaults. Dinah's existing capability check produces its Thinking option. | The toolbar will show only the values that the adapter reports for that model. |
| Claude | The [pinned ACP adapter's effort implementation](https://github.com/agentclientprotocol/claude-agent-acp/blob/v0.101.0/src/session-effort.ts) reports a select option in category thought_level when the current model reports supported effort levels. Its default and labels come from the adapter. [Claude Code](https://code.claude.com/docs/en/model-config#adjust-effort-level) also documents the effort command, launch flag and configuration. | Dinah pins adapter 0.101.0. Its ACP option is sufficient for this change; no launch flag or environment override is needed. A model without supported levels exposes no control. |
| Devin | The [Models guide](https://docs.devin.ai/cli/models#reasoning--thinking-levels) documents a separate Thinking-level setting. The [stable changelog](https://docs.devin.ai/cli/changelog/stable#v3000111) documents ACP configuration controls for model, effort and speed from 3000.11.1. | Devin is not limited to effort encoded in model variants. Older catalogues can still use names such as GPT-6 Sol High, which Dinah displays unchanged. The new control appears only when that chat actually reports thought_level. No agent upgrade or live-account inspection is included. |

The [ACP configuration protocol](https://agentclientprotocol.com/protocol/session-config-options) defines thought_level as a presentation category. The option ID remains the agent's ID. Dinah sends the selected value through its existing SetSessionOption method rather than translating it to a command-line setting.

## Persistence

The existing host saves all selected option IDs and values per chat. On reopen it applies the model first, then the remaining choices. When a model no longer offers an effort choice, the existing substitution path keeps the saved choice, uses the offered default and explains the substitution. The toolbar reads the returned options and adds no second persistence store.
