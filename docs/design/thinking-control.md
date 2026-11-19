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

## Showing the thinking

Paul reported on 20 November 2026 that thinking was not shown although Show the Agent's Thinking was on, and required that the fix hold for every agent program. Two things were wrong.

A thought was a line inside the Working block, which starts closed, so nobody saw one without opening the block. A thought is now a row of the transcript, where it arrived, with the word Thinking and its time, and the Working block has the tool calls only. Paul approved this from a mockup (docs/mockups/thinking-in-transcript). The page does this for every program alike. With the setting off no thought is shown.

Claude sent thinking for about one answer in ten. Codex and Devin send it with their steps, and the Ollama adapter passes on what a model sends. The newest Claude models leave the text of their thinking out unless asked for a summary: the API's [thinking display](https://platform.claude.com/docs/en/build-with-claude/thinking) is "omitted" by default on them. The Agent SDK documents the option `thinking: { type: "adaptive", display: "summarized" }`, and the pinned adapter takes SDK options from a session request's `_meta.claudeCode.options`, as its README says. Dinah sends that with a Claude session's request while the setting is on (`claudeSessionParams`). The same documentation says the display value does not change what a turn costs.

The option is given when a session is made or opened, and no documentation says how to change it afterwards. The API rejects adaptive thinking on the models it lists as taking extended thinking only, which in the catalogue is Haiku 4.5, and nothing says what the SDK does when a session with the option turns to such a model. Dinah therefore keeps to this:

- A chat whose saved model is one of those, or a new chat when the provider's default is one, is opened without the option.
- A session that was opened with the option is not turned to such a model as it is. When the person chooses one in the chat, the window closes the chat's session and opens it again without the option, with that model, and the chat draws its history again. This is the step Load Full History uses (`SessionLifecycle.ReopenWith`, `ReopenSessionWith`), and like it, it waits for a turn to end. The page asks the host first whether a choice needs it (`SessionOptionNeedsReopen`).
- A choice that reaches the adapter all the same, such as one a task makes for its own turn, is refused with the reason (`internal/claudeagent/thinking.go`).
- The setting reaches a chat when its session is next made or opened.

Paul asked for the reopening on 20 November 2026. The first build of that day refused the choice outright, on my mistaken belief that Dinah could not open one chat's session again while its adapter runs.

A live test (`TestLiveThinkingSummariesArriveWhenAsked`, opt-in) sends the turns to Claude: the session that asks gets its thinking and one that does not ask gets none, and the session that asked answers on Haiku 4.5 after it is closed and opened again without the request.

The adapter also reads an environment variable for the same option. No documentation mentions it, and Dinah does not use it.

