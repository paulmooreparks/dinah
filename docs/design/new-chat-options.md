# Settings a New Chat Starts With

A new chat takes its settings from three places, in this order. The person's saved defaults for the provider come first. A setting they saved nothing for takes the value in the provider table's `NewChatOptions`, when the provider's row has one for it. Every other setting keeps the value the agent reports when the session is created.

The provider table has this fact because an agent's release can change how its sessions start. The Claude adapter did so between 0.87.0 and 0.101.0: a new session with no configured permission mode had started in Manual, where Claude asks before each action that needs permission, and began to start in Auto, where Claude decides what needs permission (docs/proposals/claude-adapter-updates.md). Paul decided on 10 November 2026 that a new Claude chat with no saved mode starts in Manual. The Claude row therefore gives the `mode` setting the value `default`, which is the name Claude Code's documentation gives that mode.

The value is applied after the session is created, in the same step that applies the person's saved defaults, and the agent must confirm it. When the agent does not offer the value, the chat keeps the agent's own and the mode picker shows which it is. A reopened chat is unaffected, because it keeps the settings saved with it.

A person who wants new chats to start in another mode chooses the mode in a chat and saves it as their default, which takes precedence.
