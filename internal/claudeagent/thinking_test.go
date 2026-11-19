package claudeagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"studio/internal/acp"
)

// The API documents the models before 4.6 as rejecting adaptive thinking.
// A value with no version in it is the program's name for a current model.
func TestExtendedThinkingOnlyIsTheModelsBeforeFourSix(t *testing.T) {
	for model, want := range map[string]bool{
		"claude-haiku-4-5-20251001":  true,
		"claude-opus-4-5":            true,
		"claude-sonnet-4-5-20250929": true,
		"claude-opus-4-1-20250805":   true,
		"claude-opus-4-20250514":     true,
		"claude-3-5-sonnet-20241022": true,
		"claude-opus-4-6":            false,
		"claude-sonnet-4-6":          false,
		"claude-opus-4-6[1m]":        false,
		"claude-opus-4-7-medium":     false,
		"claude-opus-4-8":            false,
		"claude-opus-5":              false,
		"claude-opus-5-5-medium":     false,
		"claude-haiku-5-5-medium":    false,
		"claude-fable-5-2":           false,
		"default":                    false,
		"opus":                       false,
		"haiku":                      false,
		"":                           false,
	} {
		if got := ExtendedThinkingOnly(model); got != want {
			t.Errorf("ExtendedThinkingOnly(%q) = %v, want %v", model, got, want)
		}
	}
}

type sessionWithMeta struct {
	SessionID  string         `json:"sessionId,omitempty"`
	Cwd        string         `json:"cwd"`
	McpServers []any          `json:"mcpServers"`
	Meta       map[string]any `json:"_meta,omitempty"`
}

// A session that asked for thinking summaries is not let turn to a model
// that rejects the request, since nothing documented changes the request
// afterwards. Any other change of model goes to the adapter, and so does
// every change in a session that did not ask.
func TestASessionThatAsksForSummariesIsNotTurnedToAModelThatRejectsThem(t *testing.T) {
	c, _ := start(t, "")
	initialize(t, c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	set := func(session, model string) error {
		return c.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: session, ConfigID: "model", Value: model}, nil)
	}
	var created acp.NewSessionResult
	if err := c.Call(ctx, "session/new", sessionWithMeta{Cwd: "C:/W", Meta: SummariesMeta()}, &created); err != nil {
		t.Fatal(err)
	}
	if err := set(created.SessionID, "claude-haiku-4-5-20251001"); !errors.Is(err, ErrSummariesModel) {
		t.Fatalf("turning a session that asks to Haiku 4.5: %v", err)
	}
	if err := set(created.SessionID, "claude-opus-5-5-medium"); err != nil {
		t.Fatalf("turning a session that asks to a model that takes the request: %v", err)
	}
	if err := c.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: created.SessionID, ConfigID: "effort", Value: "claude-haiku-4-5-20251001"}, nil); err != nil {
		t.Fatalf("an option that is not the model: %v", err)
	}
	// A session opened without the request may use any model.
	if err := c.Call(ctx, "session/load", sessionWithMeta{SessionID: "claude:plain", Cwd: "C:/W"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := set("claude:plain", "claude-haiku-4-5-20251001"); err != nil {
		t.Fatalf("turning a session that did not ask to Haiku 4.5: %v", err)
	}
	// Once a session has asked, opening it again without the request does
	// not show that the adapter's session has lost it.
	if err := c.Call(ctx, "session/load", sessionWithMeta{SessionID: "claude:asked", Cwd: "C:/W", Meta: SummariesMeta()}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/load", sessionWithMeta{SessionID: "claude:asked", Cwd: "C:/W"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := set("claude:asked", "claude-sonnet-4-5-20250929"); !errors.Is(err, ErrSummariesModel) {
		t.Fatalf("a session that asked once, opened again without: %v", err)
	}
	// The window is told which choice needs the chat opened again, and a
	// session the adapter has closed is made anew by the next request, so it
	// no longer counts as asking.
	if !c.NeedsReopen("claude:asked", "claude-haiku-4-5-20251001") || c.NeedsReopen("claude:asked", "claude-opus-5-5-medium") || c.NeedsReopen("claude:plain", "claude-haiku-4-5-20251001") {
		t.Fatal("NeedsReopen is wrong for a session that asked, a model that takes the request, or a session that did not ask")
	}
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: "claude:asked"}, nil); err != nil {
		t.Fatal(err)
	}
	if c.NeedsReopen("claude:asked", "claude-haiku-4-5-20251001") {
		t.Fatal("a closed session still counts as asking")
	}
	if err := c.Call(ctx, "session/load", sessionWithMeta{SessionID: "claude:asked", Cwd: "C:/W"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := set("claude:asked", "claude-haiku-4-5-20251001"); err != nil {
		t.Fatalf("a session closed and opened again without the request: %v", err)
	}
}
