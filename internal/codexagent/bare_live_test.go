//go:build live

package codexagent

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
	"time"

	"studio/internal/acp"
)

// This opt-in test asks the installed Codex one small question twice, in an
// ordinary thread and in a bare one, on the person's own sign-in, and
// reports what each cost. A bare thread has to answer, and to cost less. On
// 20 November 2026 the ordinary thread cost about 26,600 tokens and the bare
// one about 9,600.
func TestLiveABareThreadCostsLess(t *testing.T) {
	if os.Getenv("DINAH_LIVE_CODEX") != "1" {
		t.Skip("set DINAH_LIVE_CODEX=1 to send live Codex turns")
	}
	codex, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	h := &testHandler{}
	c, err := Start(codex, []string{"app-server"}, t.TempDir(), h, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, nil); err != nil {
		t.Fatal(err)
	}
	cost := regexp.MustCompile(`"sessionUpdate":"turn_cost","tokens":(\d+)`)
	ask := func(bare bool) int {
		var s acp.NewSessionResult
		if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir(), McpServers: []any{}}, Bare: bare}, &s); err != nil {
			t.Fatal(err)
		}
		h.mu.Lock()
		h.events = nil
		h.mu.Unlock()
		var r acp.PromptResult
		if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: s.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: "Reply with the one word: ok."}}}, &r); err != nil {
			t.Fatal(err)
		}
		// The usage report can follow the turn's end by a moment.
		tokens, said := 0, false
		for wait := 0; wait < 20 && tokens == 0; wait++ {
			h.mu.Lock()
			for _, e := range h.events {
				if m := cost.FindSubmatch(e); m != nil {
					tokens, _ = strconv.Atoi(string(m[1]))
				}
				if bytes.Contains(e, []byte(`"agent_message_chunk"`)) && json.Valid(e) {
					said = true
				}
			}
			h.mu.Unlock()
			if tokens == 0 {
				time.Sleep(250 * time.Millisecond)
			}
		}
		if !said {
			t.Fatalf("the thread (bare %v) gave no answer", bare)
		}
		_ = c.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: s.SessionID}, nil)
		return tokens
	}
	plain, bare := ask(false), ask(true)
	t.Logf("an ordinary thread: %d tokens; a bare thread: %d tokens", plain, bare)
	if plain == 0 || bare == 0 || bare >= plain {
		t.Fatalf("a bare thread did not cost less: %d against %d", bare, plain)
	}
}
