package claudeagent

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
)

// Thinking summaries.
//
// The newest Claude models leave the text of their thinking out of an answer
// unless they are asked for a summary of it: the API's thinking.display is
// "omitted" by default on them, and "summarized" returns the summary
// (https://platform.claude.com/docs/en/build-with-claude/thinking,
// "Controlling thinking display"). The Agent SDK documents the same as its
// thinking option, { type: "adaptive", display: "summarized" }
// (https://code.claude.com/docs/en/agent-sdk/typescript, Options and
// ThinkingConfig), and the adapter takes SDK options from a session
// request's _meta.claudeCode.options, as its README says. Paul found on
// 20 November 2026 that Claude's thinking was hardly ever shown: a tenth of
// the answers had any.
//
// The option is given when a session is made or opened, and the documentation
// has no way to change it afterwards. The API rejects adaptive thinking on
// the models it lists as taking extended thinking only, which are Haiku 4.5,
// Opus 4.5 and Sonnet 4.5, and nothing says what the SDK does when a session
// that has the option turns to one of them. So a session that was opened
// with the option is not let turn to such a model: the choice is refused
// with the reason and the way round. The adapter has an environment variable
// for the same option, which no documentation mentions, and it is not used.

// SummariesMeta is the _meta of a session request that asks Claude for
// summaries of its thinking.
func SummariesMeta() map[string]any {
	return map[string]any{"claudeCode": map[string]any{"options": map[string]any{
		"thinking": map[string]any{"type": "adaptive", "display": "summarized"},
	}}}
}

var modelVersion = regexp.MustCompile(`^claude-(?:[a-z]+-)?(\d+)(?:-(\d{1,2}))?(?:[^0-9]|$)`)

// ExtendedThinkingOnly says a model option's value is a model the API
// documents as rejecting adaptive thinking: one before the 4.6 models. A
// value that gives no version, such as "default" or "opus", is the program's
// name for a current model and is not one of them.
func ExtendedThinkingOnly(model string) bool {
	m := modelVersion.FindStringSubmatch(model)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major < 4 || (major == 4 && minor < 6)
}

// ErrSummariesModel is why a chat that asks for thinking summaries is not
// let turn to a model that rejects the request.
var ErrSummariesModel = errors.New("That model does not take the request for a summary of its thinking, which this chat made of Claude when it was opened. To use the model, turn off Show the Agent's Thinking in Settings and restart Claude.")

// asksSummaries says a session request asks for thinking summaries.
func asksSummaries(params json.RawMessage) bool {
	var p struct {
		Meta struct {
			ClaudeCode struct {
				Options struct {
					Thinking struct {
						Display string `json:"display"`
					} `json:"thinking"`
				} `json:"options"`
			} `json:"claudeCode"`
		} `json:"_meta"`
	}
	return json.Unmarshal(params, &p) == nil && p.Meta.ClaudeCode.Options.Thinking.Display == "summarized"
}

// noteSession keeps whether the session a request made or opened asks for
// thinking summaries. params and result have the adapter's own session IDs.
func (c *Conn) noteSession(params, result json.RawMessage) {
	var id struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(params, &id) != nil || id.SessionID == "" {
		if json.Unmarshal(result, &id) != nil || id.SessionID == "" {
			return
		}
	}
	// Once asked, a session counts as asking for as long as this adapter
	// runs. The adapter keeps a session it already has when the session is
	// opened again, so a later request without the option does not show that
	// the session has lost it.
	if asksSummaries(params) {
		c.summaries.Store(id.SessionID, true)
	}
}

// refusedModel is ErrSummariesModel when a request would turn a session that
// asks for thinking summaries to a model that rejects them.
func (c *Conn) refusedModel(params json.RawMessage) error {
	var p struct {
		SessionID string `json:"sessionId"`
		ConfigID  string `json:"configId"`
		Value     any    `json:"value"`
	}
	if json.Unmarshal(params, &p) != nil || p.ConfigID != "model" {
		return nil
	}
	model, _ := p.Value.(string)
	if _, asks := c.summaries.Load(p.SessionID); asks && ExtendedThinkingOnly(model) {
		return ErrSummariesModel
	}
	return nil
}
