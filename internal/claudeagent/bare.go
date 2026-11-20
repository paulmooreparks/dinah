package claudeagent

// A bare session.
//
// A judgement is one small question, and Claude Code gave it what it gives
// every session: its instructions for a coding agent, the schemas of its
// tools, and the person's own settings with their instructions, servers and
// hooks. A judgement through Claude cost about 42,800 tokens whatever was
// asked (the judgement records, November 2026). A bare session leaves that
// out with two options of the Agent SDK, which the adapter takes from a
// session request's _meta.claudeCode.options as its README says:
// systemPrompt as a text of its own, which the SDK's documentation says
// replaces the default entirely, and settingSources as an empty list, which
// it says disables user, project and local settings
// (https://code.claude.com/docs/en/agent-sdk/typescript, Options, and
// https://code.claude.com/docs/en/agent-sdk/modifying-system-prompts).
//
// An empty list of tools removes the tools' schemas, which were about
// 34,000 of the tokens: with it the same question cost 911. The
// documentation says tools takes a list of tool names and does not say what
// an empty list does. Paul decided on 20 November 2026 to use it all the
// same for judgement calls, which are meant to be quick and are not
// critical. If a later release treats an empty list otherwise, the opt-in
// live test shows it as a jump in what a bare session uses.

// bareInstructions take the place of Claude Code's instructions.
const bareInstructions = "Answer the one message you are given, in the form it asks for. Use no tools and take no action."

// BareMeta is the _meta of a session request for one question that is to be
// answered from the question alone.
func BareMeta() map[string]any {
	return map[string]any{"claudeCode": map[string]any{"options": map[string]any{
		"systemPrompt":   bareInstructions,
		"tools":          []string{},
		"settingSources": []string{},
	}}}
}
