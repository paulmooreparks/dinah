package ollamaagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/acp"
)

// toolLoop is an Ollama whose model asks for a tool in each of its first
// toolRounds answers, and then answers in words.
func toolLoop(t *testing.T, toolRounds int) (*Conn, *host, string, *atomic.Int64) {
	t.Helper()
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			io.WriteString(w, `{"version":"0.34.1"}`)
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"test","model":"test"}]}`)
		case "/api/show":
			io.WriteString(w, `{"capabilities":["completion","tools"]}`)
		case "/api/chat":
			var req chatRequest
			json.NewDecoder(r.Body).Decode(&req)
			if int(requests.Add(1)) <= toolRounds {
				io.WriteString(w, `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"read_file","arguments":{"path":"notes.md"}}}]},"done":true}`+"\n")
				return
			}
			io.WriteString(w, `{"message":{"role":"assistant","content":"Done."},"done":true}`+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "notes.md"), []byte("notes"), 0o600)
	h := &host{}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	var opened acp.NewSessionResult
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}}, &opened); err != nil {
		t.Fatal(err)
	}
	return c, h, opened.SessionID, &requests
}

func runTurn(t *testing.T, c *Conn, id string) acp.PromptResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var res acp.PromptResult
	if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: id, Prompt: []acp.ContentBlock{{Type: "text", Text: "Keep reading."}}}, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

// With no limit, which is how every install starts, a turn takes as many rounds
// as the model asks for, past the 60 that used to stop it.
func TestATurnWithNoRoundLimitTakesAsManyRoundsAsTheModelAsksFor(t *testing.T) {
	c, h, id, requests := toolLoop(t, 75)
	if res := runTurn(t, c, id); res.StopReason != "end_turn" || requests.Load() != 76 || strings.Contains(h.said(), "limit of") {
		t.Fatalf("stop %q after %d requests: %s", res.StopReason, requests.Load(), h.said())
	}
}

// A limit the person set ends the turn with the protocol's own reason and a
// sentence saying it was that limit, and the next message continues the chat.
func TestATurnStopsAtTheLimitThePersonSet(t *testing.T) {
	c, h, id, requests := toolLoop(t, 100)
	c.SetRoundLimit(func() int { return 3 })
	res := runTurn(t, c, id)
	if res.StopReason != "max_turn_requests" || requests.Load() != 3 {
		t.Fatalf("stop %q after %d requests", res.StopReason, requests.Load())
	}
	if !strings.Contains(h.said(), "The turn stopped at the limit of 3 rounds of tool calls that you set in Settings. Send a message to continue.") {
		t.Fatalf("the turn did not say why it stopped: %s", h.said())
	}
	c.SetRoundLimit(func() int { return 0 })
	if res := runTurn(t, c, id); requests.Load() <= 3 || res.StopReason == "max_turn_requests" {
		t.Fatalf("the chat did not continue: stop %q after %d requests", res.StopReason, requests.Load())
	}
}

// The limit is asked before each round, so a change made while a turn runs
// applies to it, and a limit of zero is none.
func TestAChangedLimitAppliesToATurnThatIsRunning(t *testing.T) {
	c, _, id, requests := toolLoop(t, 100)
	var limit atomic.Int64
	c.SetRoundLimit(func() int {
		if requests.Load() >= 2 {
			limit.Store(4)
		}
		return int(limit.Load())
	})
	if res := runTurn(t, c, id); res.StopReason != "max_turn_requests" || requests.Load() != 4 {
		t.Fatalf("stop %q after %d requests", res.StopReason, requests.Load())
	}
}
