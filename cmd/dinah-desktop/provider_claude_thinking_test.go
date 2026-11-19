package main

import (
	"encoding/json"
	"strings"
	"testing"

	"studio/internal/acp"
)

// With Show the Agent's Thinking on, a Claude session is made or opened with
// the documented request for summaries of its thinking, unless the chat's
// model is one that rejects the request. With the setting off no session
// asks.
func TestClaudeSessionsAskForThinkingSummaries(t *testing.T) {
	a := &App{}
	a.settings.ShowThinking = true
	a.settings.SessionOptions = map[string]map[string]json.RawMessage{
		"claude:old":     {"model": json.RawMessage(`"claude-haiku-4-5-20251001"`)},
		"claude:current": {"model": json.RawMessage(`"claude-sonnet-5-5-medium"`)},
	}
	params := func(id string) string {
		b, err := json.Marshal(claudeSessionParams(a, acp.LoadSessionParams{SessionID: id, Cwd: "C:/W", McpServers: []any{}}))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	const asks = `"_meta":{"claudeCode":{"options":{"thinking":{"display":"summarized","type":"adaptive"}}}}`
	if got := params(""); !strings.Contains(got, asks) || strings.Contains(got, "sessionId") || !strings.Contains(got, `"cwd":"C:/W"`) {
		t.Errorf("a new session: %s", got)
	}
	if got := params("claude:current"); !strings.Contains(got, asks) || !strings.Contains(got, `"sessionId":"claude:current"`) {
		t.Errorf("a chat on a current model: %s", got)
	}
	if got := params("claude:unsaved"); !strings.Contains(got, asks) {
		t.Errorf("a chat with no saved model: %s", got)
	}
	if got := params("claude:old"); strings.Contains(got, "_meta") {
		t.Errorf("a chat on a model that rejects the request: %s", got)
	}
	// The provider's default is the model of a chat with no choice of its own.
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"claude": {"model": json.RawMessage(`"claude-haiku-4-5-20251001"`)}}
	if got := params(""); strings.Contains(got, "_meta") {
		t.Errorf("a new session when the default model rejects the request: %s", got)
	}
	if got := params("claude:current"); !strings.Contains(got, asks) {
		t.Errorf("a chat whose own model takes the request, whatever the default: %s", got)
	}
	a.settings.ProviderDefaults = nil
	a.settings.ShowThinking = false
	for _, id := range []string{"", "claude:current", "claude:old"} {
		if got := params(id); strings.Contains(got, "_meta") {
			t.Errorf("with the setting off, %q: %s", id, got)
		}
	}
}
