package ollamaagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"studio/internal/acp"
)

// fakeCommands answers each prompt of a conversation with the next command
// line, so that a chat runs several different commands in turn.
func fakeCommands(t *testing.T, lines ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			io.WriteString(w, `{"version":"0.34.1"}`)
		case "/api/tags":
			io.WriteString(w, `{"models":[{"name":"nano:4b","model":"nano:4b","details":{"parameter_size":"4.0B"}}]}`)
		case "/api/show":
			io.WriteString(w, `{"capabilities":["completion","tools"],"model_info":{"general.architecture":"nano","nano.context_length":32768}}`)
		case "/api/chat":
			var req chatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			last := req.Messages[len(req.Messages)-1]
			if last.Role != "user" {
				io.WriteString(w, `{"message":{"role":"assistant","content":"done"},"done":true,"done_reason":"stop"}`+"\n")
				return
			}
			turn := 0
			for _, m := range req.Messages {
				if m.Role == "user" {
					turn++
				}
			}
			args, _ := json.Marshal(map[string]any{"command": lines[min(turn-1, len(lines)-1)], "path": "notes.md"})
			io.WriteString(w, `{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"run_command","arguments":`+string(args)+`}}]},"done":true,"done_reason":"stop"}`+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// How often a chat's command asks the person: once in the chat by default,
// which the person's answer then covers for every command and for a
// reconnect; every command line when chosen; and never when chosen.
func TestWhenACommandAsksThePerson(t *testing.T) {
	cases := []struct {
		name   string
		policy string
		asked  int
	}{
		{"once in a chat", ApprovalsChat, 1},
		{"once in a chat by default", "", 1},
		{"every command line", ApprovalsEvery, 3},
		{"never", ApprovalsNone, 0},
	}
	lines := []string{"echo one > a.txt", "echo two > b.txt", "echo three > c.txt"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fakeCommands(t, lines...)
			ws := t.TempDir()
			os.WriteFile(filepath.Join(ws, "notes.md"), []byte("kept"), 0o644)
			h := &host{answer: "always"}
			c, err := Start(srv.URL, t.TempDir(), h)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			ctx := context.Background()
			var opened acp.NewSessionResult
			if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}, CommandApprovals: tc.policy}, &opened); err != nil {
				t.Fatal(err)
			}
			prompt := func(text string) {
				t.Helper()
				if err := c.Call(ctx, "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: text}}}, nil); err != nil {
					t.Fatal(err)
				}
			}
			prompt("one")
			prompt("two")
			// A chat that reconnects in the same run keeps what was allowed.
			if err := c.Call(ctx, "session/load", SessionParams{LoadSessionParams: acp.LoadSessionParams{SessionID: opened.SessionID, Cwd: ws}, CommandApprovals: tc.policy}, nil); err != nil {
				t.Fatal(err)
			}
			prompt("three")

			h.mu.Lock()
			asked := len(h.asked)
			h.mu.Unlock()
			if asked != tc.asked {
				t.Fatalf("asked %d times, want %d", asked, tc.asked)
			}
			for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
				if _, err := os.Stat(filepath.Join(ws, name)); err != nil {
					t.Errorf("%s was not written, so a command the person allowed did not run", name)
				}
			}
		})
	}
}

// A refusal applies to the rest of this turn, even when a retry changes
// the command or asks for a different privileged tool. A new turn can ask again.
func TestRefusalStopsPermissionRequestsUntilNextTurn(t *testing.T) {
	for _, policy := range []string{ApprovalsChat, ApprovalsEvery} {
		t.Run(policy, func(t *testing.T) {
			h := &host{answer: "reject"}
			c := &Conn{handler: h}
			var mu sync.Mutex
			var refusal string
			tt := turnTools{c: c, commands: policy, allowed: map[string]bool{}, mu: &mu, refusal: &refusal}
			for _, key := range []string{"cmd", "cmd-changed", "write", "applet"} {
				if allowed, why := tt.allow(context.Background(), key, key, key, nil); allowed || why == "" {
					t.Fatalf("%s was allowed or lost the refusal reason", key)
				}
			}
			if len(h.asked) != 1 {
				t.Fatalf("asked %d times after refusal, want 1", len(h.asked))
			}
			if _, ok := list(t.TempDir()); !ok {
				t.Fatal("read-only tools must remain available")
			}
			// prompt creates a fresh refusal value for each user message.
			var nextRefusal string
			tt.refusal = &nextRefusal
			h.answer = "always"
			if allowed, _ := tt.allow(context.Background(), "next", "cmd", "next command", nil); !allowed {
				t.Fatal("a new user message could not request permission")
			}
			if len(h.asked) != 2 {
				t.Fatalf("asked %d times across two turns, want 2", len(h.asked))
			}
		})
	}
}
