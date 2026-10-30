# Draft Issue for claude-agent-acp: A Session's Model List Can Lack a Model It Was Started With

This is a draft for Paul to file at https://github.com/agentclientprotocol/claude-agent-acp/issues under his own account. Nothing here has been filed. Dinah Development wrote it on 28 October 2026 from the diagnosis in `docs/design/chat-recovery.md`.

---

**Title:** A loaded session's model option can omit the model the session used, and is never refreshed

**Version:** `@agentclientprotocol/claude-agent-acp` 0.87.0. The same code path is present in 0.99.0.

**What happens.** An ACP client saves the model a person chose for a chat, for example the value `fable` from the `model` config option, and on reopening the chat calls `session/load`, then checks that the saved value is among the values the `model` option offers before restoring it with `session/set_config_option`. Some of the time, the `model` option returned for the loaded session does not include `fable`, although the session ran with a Fable model and other sessions created minutes earlier or later offer it. The client cannot then restore the person's choice, because ACP allows only offered values, and the conversation cannot continue with the model it was using.

**When.** It is intermittent. On one machine on 28 October 2026 it happened eight times across three chats, each saved with `fable`; chats saved with `default` or `opus` never failed. Retrying the load seconds later sometimes succeeded.

**What the adapter appears to do.** As far as we can read it, the adapter builds a session's model option once, at load, from the model list the Claude Code CLI reports at startup, and never rebuilds it. Later `config_option_update` notifications are sent when the mode, the model or fast mode changes, and they repeat the original list. The CLI's list seems to depend on a server-supplied set of extra models that it refreshes in the background and caches, which can list a Fable model by its full name (for example `claude-fable-5-1[1m]`) in place of the `fable` alias. We have not relied on any of this; it is offered only to help locate the cause.

**What would let clients restore a person's choice reliably.** Any one of these, documented:

1. The loaded session's `model` option always includes the model the session last used, under the value the client saved.
2. The adapter refreshes a session's model list when the CLI's list changes, through the SDK's documented `supportedModels()`, and announces the new list with a `config_option_update` notification, so that a client can wait a bounded time for a saved value to appear.
3. The documentation says which values are stable across restarts (aliases or full model names), so that a client saves the stable one.

**Why it matters.** A client that silently switches a reopened conversation to a different model would change the conversation without the person knowing, so a careful client refuses instead, and the person loses the use of the chat until they choose another model by hand.
