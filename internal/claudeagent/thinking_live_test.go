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
}
