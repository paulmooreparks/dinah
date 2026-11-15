# Effort changes during a running turn

This note answers Team Lead request 9548 from official documentation read on 16 November 2026. It describes documented external contracts. It does not infer timing from a successful experiment or prescribe new product behavior. The cross-project Architectural Principles document remains the source of the documentation requirement.

| Program and interface | What the documentation establishes | What the control may promise from that contract |
| --- | --- | --- |
| Codex App Server | Effort is a turn/start override. The override becomes the default for later turns. turn/steer adds input to the current turn and accepts no turn-level overrides. | A selected effort sent with the next turn/start applies to that new turn. A steering message cannot change the active turn's effort. |
| Claude Code terminal | After /effort confirms, the next message uses the new level. A message queued while Claude works can be consumed after current tool calls finish, or at turn end at the latest. | The terminal documentation promises the next message, which can affect the continuation of work already in progress. It does not promise that the whole active turn stays at its old effort. |
| Claude through ACP | ACP documents setting session configuration and returning complete updated options. The adapter README documents protocol support. Neither document specifies when an effort change submitted during a prompt takes effect. | The ACP control cannot claim a next-turn boundary from those documents. The terminal command's timing is not an ACP method contract. |
| Devin through ACP | Devin documents cycling thinking levels during a session. ACP defines session/set_config_option and the returned configuration state, but neither specifies the timing of a thinking-level change during an active prompt. | The control must say that the program does not specify when a running-turn change takes effect. |
| Ollama HTTP | The think field belongs to each chat or generate request. Supported values and defaults come from the model's show response. The documented API has no operation for replacing think in an already submitted streaming request. | A new request carries the new choice. Calling that the next application turn requires an additional host policy, because an agent turn can contain more than one HTTP request. |

Claude's terminal rule is more precise than a generic next-turn label. The documentation permits a queued message to take effect at a tool boundary within ongoing work. For the ACP programs, the absence of a stated boundary must remain visible instead of being filled in from that terminal behavior.

The Ollama conclusion about subsequent requests follows from its request-scoped parameter. Its documentation does not state that an application-level multi-request turn snapshots the parameter. This note makes no claim about Dinah's existing adapter implementation.

## Sources

The official [Codex App Server documentation](https://learn.chatgpt.com/docs/app-server#turns) defines per-turn overrides and the separate [steer operation](https://learn.chatgpt.com/docs/app-server#steer-an-active-turn). The [Claude Code model configuration documentation](https://code.claude.com/docs/en/model-config#when-a-new-effort-level-starts) gives the next-message and tool-boundary behavior. The [Claude ACP adapter README](https://github.com/agentclientprotocol/claude-agent-acp/blob/main/README.md) lists its protocol surface without an effort timing guarantee.

The [Devin model documentation](https://docs.devin.ai/cli/models#reasoning--thinking-levels) describes the session thinking selector. The [ACP configuration specification](https://agentclientprotocol.com/protocol/v1/session-config-options) defines the configuration request and full-state response without an active-prompt effect boundary. The [Ollama thinking documentation](https://docs.ollama.com/capabilities/thinking) defines think on individual HTTP requests and model-specific supported values.

No provider was contacted, program launched or account accessed for this documentation review. No product code changed.
