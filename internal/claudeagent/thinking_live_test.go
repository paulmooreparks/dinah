//go:build live

package claudeagent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

type thoughts struct {
	mu    sync.Mutex
	chars map[string]int
	said  map[string]int
}

func (r *thoughts) Notify(method string, params json.RawMessage) {
	var p struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string `json:"sessionUpdate"`
			Content       struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	if method != "session/update" || json.Unmarshal(params, &p) != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch p.Update.SessionUpdate {
	case "agent_thought_chunk":
		r.chars[p.SessionID] += len(p.Update.Content.Text)
	case "agent_message_chunk":
		r.said[p.SessionID] += len(p.Update.Content.Text)
	}
}

func (r *thoughts) Request(context.Context, string, json.RawMessage) (any, *acp.Error) {
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
}

// This opt-in test sends two short turns to Claude through the installed,
// pinned adapter on the person's own Claude Code sign-in: one in a session
// that asks for thinking summaries and one in a session that does not. It
// shows that the request reaches the model, which a stand-in cannot. The
// second turn is only reported, since a model may send its thinking unasked.
// DINAH_LIVE_STATE is the folder the adapter is installed under.
func TestLiveThinkingSummariesArriveWhenAsked(t *testing.T) {
	if os.Getenv("DINAH_LIVE_CLAUDE") != "1" {
		t.Skip("set DINAH_LIVE_CLAUDE=1 to send live Claude turns")
	}
	state := os.Getenv("DINAH_LIVE_STATE")
	if !InstalledRelease(state, Version) {
		t.Fatalf("the pinned adapter is not installed under %q", state)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	r := &thoughts{chars: map[string]int{}, said: map[string]int{}}
	c, err := Start(node, state, t.TempDir(), "", r, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
		t.Fatal(err)
	}
	var asking, askingCwd string
	turn := func(meta map[string]any) (thought, said int) {
		cwd := filepath.Join(t.TempDir(), "w")
		os.MkdirAll(cwd, 0o700)
		var created acp.NewSessionResult
		if err := c.Call(ctx, "session/new", sessionWithMeta{Cwd: cwd, McpServers: []any{}, Meta: meta}, &created); err != nil {
			t.Fatal(err)
		}
		prompt := "Use no tools. Which is larger, 37 to the power 5 or 5 to the power 11? Work it out carefully before you answer, then answer in one short line."
		if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: created.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: prompt}}}, nil); err != nil {
			t.Fatal(err)
		}
		if meta != nil {
			asking, askingCwd = created.SessionID, cwd
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.chars[created.SessionID], r.said[created.SessionID]
	}
	asked, saidAsked := turn(SummariesMeta())
	plain, saidPlain := turn(nil)
	t.Logf("asked for summaries: %d characters of thinking, %d of answer; not asked: %d of thinking, %d of answer", asked, saidAsked, plain, saidPlain)
	if saidAsked == 0 {
		t.Fatal("the session that asked gave no answer")
	}
	if asked == 0 {
		t.Fatal("the session that asked for summaries sent no thinking")
	}
	// The session that asked is turned to a model that rejects the request
	// as the window does it: closed, opened again without the request, and
	// only then given the model. A turn on that model then has to answer.
	const old = "claude-haiku-4-5-20251001"
	if !c.NeedsReopen(asking, old) {
		t.Fatal("the session that asked does not need opening again for a model that rejects the request")
	}
	if err := c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: asking}, nil); err != nil {
		t.Fatalf("closing the session: %v", err)
	}
	if err := c.Call(ctx, "session/load", sessionWithMeta{SessionID: asking, Cwd: askingCwd, McpServers: []any{}}, nil); err != nil {
		t.Fatalf("opening the session again: %v", err)
	}
	if err := c.Call(ctx, "session/set_config_option", acp.SetConfigOptionParams{SessionID: asking, ConfigID: "model", Value: old}, nil); err != nil {
		t.Fatalf("choosing %s after opening again: %v", old, err)
	}
	r.mu.Lock()
	before := r.said[asking]
	r.mu.Unlock()
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: asking, Prompt: []acp.ContentBlock{{Type: "text", Text: "Use no tools. Reply with the one word: done."}}}, nil); err != nil {
		t.Fatalf("a turn on %s after opening again: %v", old, err)
	}
	r.mu.Lock()
	after := r.said[asking]
	r.mu.Unlock()
	t.Logf("opened again without the request and turned to %s: %d characters of answer", old, after-before)
	if after == before {
		t.Fatalf("the turn on %s gave no answer", old)
	}
}
