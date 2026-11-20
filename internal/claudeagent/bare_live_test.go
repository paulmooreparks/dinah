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

type usageWatch struct {
	mu   sync.Mutex
	used map[string]int
	said map[string]int
}

func (w *usageWatch) Notify(method string, params json.RawMessage) {
	var p struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			Kind    string `json:"sessionUpdate"`
			Used    *int   `json:"used"`
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	if method != "session/update" || json.Unmarshal(params, &p) != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if p.Update.Kind == "usage_update" && p.Update.Used != nil {
		w.used[p.SessionID] = *p.Update.Used
	}
	if p.Update.Kind == "agent_message_chunk" {
		w.said[p.SessionID] += len(p.Update.Content.Text)
	}
}

func (w *usageWatch) Request(context.Context, string, json.RawMessage) (any, *acp.Error) {
	return map[string]any{"outcome": map[string]any{"outcome": "cancelled"}}, nil
}

// This opt-in test asks Claude one small question in an ordinary session and
// in a bare one, through the installed adapter on the person's own sign-in,
// and reports what each used. A bare session has to answer, and to use less.
func TestLiveABareSessionUsesLess(t *testing.T) {
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
	w := &usageWatch{used: map[string]int{}, said: map[string]int{}}
	c, err := Start(node, state, t.TempDir(), "", w, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
		t.Fatal(err)
	}
	ask := func(meta map[string]any) (used, said int) {
		cwd := filepath.Join(t.TempDir(), "w")
		os.MkdirAll(cwd, 0o700)
		var created acp.NewSessionResult
		if err := c.Call(ctx, "session/new", sessionWithMeta{Cwd: cwd, McpServers: []any{}, Meta: meta}, &created); err != nil {
			t.Fatal(err)
		}
		if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: created.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Reply with the one word: ok."}}}, nil); err != nil {
			t.Fatal(err)
		}
		time.Sleep(500 * time.Millisecond)
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.used[created.SessionID], w.said[created.SessionID]
	}
	plain, saidPlain := ask(nil)
	bare, saidBare := ask(BareMeta())
	t.Logf("an ordinary session used %d and said %d characters; a bare session used %d and said %d characters", plain, saidPlain, bare, saidBare)
	if saidBare == 0 {
		t.Fatal("the bare session gave no answer")
	}
	if plain == 0 || bare == 0 || bare >= plain {
		t.Fatalf("a bare session did not use less: %d against %d", bare, plain)
	}
}
