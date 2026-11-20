
## What a call costs, and bare sessions (20 November 2026)

The judgement records showed that a call through an agent program cost tens of thousands of tokens whatever was asked: about 33,000 through Codex and about 42,800 through Claude. The question is capped at 8,000 characters, so nearly all of it was what the program gives every session: its instructions for a coding agent, its tools' schemas, and the person's own memories, skills, plugins and settings.

A judgement call now starts a bare session where the program documents how.

- Codex (`internal/codexagent/bare.go`): thread/start with instructions of its own and `ephemeral`, and the configuration keys that turn off web search, project instructions, the skills' texts, the shell and image tools, connectors, the multi-agent tools, memories and each installed plugin. Measured with the installed Codex, one small question went from about 26,600 tokens to about 9,600. The person's memories were the largest part.
- Claude (`internal/claudeagent/bare.go`): a system prompt of its own, no setting sources and an empty list of tools, through the session request's `_meta.claudeCode.options`. Measured, one small question went from about 54,000 tokens to 911. The first two options alone took it to about 34,800; the rest is the tools' schemas.

The SDK's documentation says that `tools` takes a list of tool names and does not say what an empty list does. Paul decided on 20 November 2026 to use it all the same: judgement calls are meant to be quick and are not critical. The live test would show a later release treating it otherwise as a jump in cost.

Devin reports no cost for a call, so nothing is known there, and Ollama's calls cost a few hundred tokens already.

Both measurements are kept as opt-in live tests (`TestLiveABareThreadCostsLess`, `TestLiveABareSessionUsesLess`).
