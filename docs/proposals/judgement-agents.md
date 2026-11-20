
## What a call costs, and bare sessions (20 November 2026)

The judgement records showed that a call through an agent program cost tens of thousands of tokens whatever was asked: about 33,000 through Codex and about 42,800 through Claude. The question is capped at 8,000 characters, so nearly all of it was what the program gives every session: its instructions for a coding agent, its tools' schemas, and the person's own memories, skills, plugins and settings.

A judgement call now starts a bare session where the program documents how.

- Codex (`internal/codexagent/bare.go`): thread/start with instructions of its own and `ephemeral`, and the configuration keys that turn off web search, project instructions, the skills' texts, the shell and image tools, connectors, the multi-agent tools, memories and each installed plugin. Measured with the installed Codex, one small question went from about 26,600 tokens to about 9,600. The person's memories were the largest part.
- Claude (`internal/claudeagent/bare.go`): a system prompt of its own, no setting sources and an empty list of tools, through the session request's `_meta.claudeCode.options`. Measured, one small question went from about 54,000 tokens to 911. The first two options alone took it to about 34,800; the rest is the tools' schemas.

The SDK's documentation says that `tools` takes a list of tool names and does not say what an empty list does. Paul decided on 20 November 2026 to use it all the same: judgement calls are meant to be quick and are not critical. The live test would show a later release treating it otherwise as a jump in cost.

Devin reports no cost for a call, so nothing is known there, and Ollama's calls cost a few hundred tokens already.

Both measurements are kept as opt-in live tests (`TestLiveABareThreadCostsLess`, `TestLiveABareSessionUsesLess`).

## The first-start question (20 November 2026)

Judgements were on from the first start and did nothing a person could see, since no model was chosen, and a calibration run then chose one in the background without a word. Paul decided that judgements ask first. Until the person answers, the state is "setup": the bottom bar's item says Set Up, the flyout asks the question in place of its facts and opens by itself once, no model is chosen and no call is made, calibration included (`cmd/dinah-desktop/judge_setup.go`, mockup in docs/mockups/judgements-first-start).

Turn On chooses the default provider when it can judge, sets the starting budget and starts the choice of the model. Not Now keeps the question in the flyout. An installation that already has a model was set up and is not asked. The answer is kept in judgements.json as `setup`, which the page never writes.

The starting budget is 200 calls a day, about four times what a busy day used in the records, turned into the provider's unit for a week from what a bare call was measured to cost (`JudgeCallCost` in the provider table): 1,000 tokens through Claude and 10,000 through Codex. A local judge has no budget, and neither has a provider whose calls report no cost.
