//go:build live

// The live tests run against real Devin. TestOptions costs nothing; the
// others run short turns and cost a few Devin credits. They run only with
// the live tag:
//
//	go test -tags live -run TestOptions -v .
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/agents"
)

// liveApp is an app on a throwaway workspace, with the person's saved
// settings put back afterwards.
func liveApp(t *testing.T) (*App, func()) {
	ws, err := os.MkdirTemp("", "dinah-live-")
	if err != nil {
		t.Fatal(err)
	}
	// The app's settings live in a folder of the test's own, so that a test
	// never changes the settings of the Dinah the person uses, even
	// while it runs or when it is killed.
	configOverride = filepath.Join(ws, ".test-config")
	a := NewApp()
	a.settings.Workspace = ws
	a.settings.Approvals = ApprovalsDinah
	return a, func() {
		a.Stop()
		time.Sleep(2 * time.Second)
		_ = os.RemoveAll(ws)
	}
}

// recorder collects what the app emits.
type recorder struct {
	mu    sync.Mutex
	said  strings.Builder
	turns []string
	tool  chan struct{}
}

func record(a *App) *recorder {
	r := &recorder{tool: make(chan struct{}, 1)}
	a.sink = func(name string, data any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		switch name {
		case "agent:update":
			raw := data.(map[string]any)["update"].(json.RawMessage)
			var u struct {
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Text string `json:"text"`
				} `json:"content"`
			}
			_ = json.Unmarshal(raw, &u)
			if u.SessionUpdate == "agent_message_chunk" {
				r.said.WriteString(u.Content.Text)
			}
			if u.SessionUpdate == "tool_call" {
				select {
				case r.tool <- struct{}{}:
				default:
				}
			}
		case "agent:permission":
			m := data.(map[string]any)
			go a.Answer(m["key"].(string), "allow_once")
		case "agent:turn":
			r.turns = append(r.turns, data.(map[string]any)["stopReason"].(string))
		}
	}
	return r
}

func waitIdle(t *testing.T, a *App, id string) {
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		if s := a.session(id); s == nil || !s.Info().Working {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("the turn did not end within 4 minutes")
}

// TestOptions opens a session, which costs nothing, changes its model to
// another value Devin offers, and checks that Devin reports the change.
func TestOptions(t *testing.T) {
	a, done := liveApp(t)
	defer done()
	s, err := a.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range a.SessionOptions(s.ID) {
		t.Logf("option %s (%s) category=%s current=%s, %d values", o.ID, o.Name, o.Category, o.CurrentValue, len(agents.OptionValues(o)))
		if o.Category != "model" {
			continue
		}
		var cur, next string
		_ = json.Unmarshal(o.CurrentValue, &cur)
		for _, v := range agents.OptionValues(o) {
			if v != cur {
				next = v
				break
			}
		}
		val, _ := json.Marshal(next)
		after, err := a.SetSessionOption(s.ID, o.ID, val)
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range after {
			if x.ID == o.ID {
				var got string
				_ = json.Unmarshal(x.CurrentValue, &got)
				if got != next {
					t.Fatalf("Devin reports %q after setting %q", got, next)
				}
				t.Logf("model changed from %s to %s", cur, got)
				return
			}
		}
		t.Fatal("the model option was missing after the change")
	}
	t.Fatal("Devin offered no model option")
}

// TestAttach sends one prompt with a pasted image and a dropped text file,
// and checks that Devin read both: the word in the image and the number in
// the file. It runs one short turn.
func TestAttach(t *testing.T) {
	a, done := liveApp(t)
	defer done()
	r := record(a)
	s, err := a.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	png, err := os.ReadFile("testdata/word.png")
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	_, init := a.providerInitLocked("devin")
	a.mu.Unlock()
	t.Logf("prompt capabilities: %+v", init.AgentCapabilities.PromptCapabilities)
	// DINAH_MODEL picks the session's model, to tell a model that cannot see
	// images from an image that did not arrive.
	if m := os.Getenv("DINAH_MODEL"); m != "" {
		v, _ := json.Marshal(m)
		if _, err := a.SetSessionOption(s.ID, "model", v); err != nil {
			t.Fatal(err)
		}
		t.Logf("model: %s", m)
	}
	atts := []Attachment{
		{Kind: "file", Name: "word.png", MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(png)},
		{Kind: "file", Name: "answer.eml", MimeType: "message/rfc822", Data: base64.StdEncoding.EncodeToString([]byte("From: Parser team\nSubject: Retry interval\n\nWe want a retry interval of 73 seconds.\n"))},
	}
	if err := a.Send(s.ID, "Reply with only two things separated by a space: the word written in the attached image, and the number of seconds in the attached email. Do not run any tools.", nil, atts); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, a, s.ID)
	r.mu.Lock()
	defer r.mu.Unlock()
	said := strings.ToUpper(r.said.String())
	t.Logf("Devin said: %q", r.said.String())
	if !strings.Contains(said, "PELICAN") || !strings.Contains(said, "73") {
		t.Fatalf("Devin did not read both attachments: %q", r.said.String())
	}
}

// TestDelete opens a session, which costs nothing, removes it with
// session/delete, and checks that Devin no longer lists it.
func TestDelete(t *testing.T) {
	a, done := liveApp(t)
	defer done()
	s, err := a.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteSession(s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err := a.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range list {
		if x.ID == s.ID {
			t.Fatalf("session %s is still listed after it was removed", s.ID)
		}
	}
	t.Logf("removed %s; %d sessions listed for the workspace", s.ID, len(list))
}

// TestSteer starts a slow task, steers the agent while it works, and checks
// that the next turn follows the steering message in the same session.
func TestSteer(t *testing.T) {
	a, done := liveApp(t)
	defer done()
	r := record(a)
	s, err := a.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Send(s.ID, "Run the shell command `ping -n 30 127.0.0.1` and wait for it to finish, then reply with only the word apple.", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-r.tool:
	case <-time.After(3 * time.Minute):
		t.Fatal("no tool call within 3 minutes")
	}
	time.Sleep(2 * time.Second)
	r.mu.Lock()
	r.said.Reset()
	r.mu.Unlock()
	if err := a.Steer(s.ID, "Forget the ping and the word apple. Reply with only the word banana.", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, a, s.ID)
	r.mu.Lock()
	defer r.mu.Unlock()
	t.Logf("turns: %v; said after steering: %q", r.turns, r.said.String())
	if len(r.turns) < 2 || r.turns[0] != "steered" {
		t.Fatalf("expected a steered turn and then a finished one, got %v", r.turns)
	}
	if said := strings.ToLower(r.said.String()); !strings.Contains(said, "banana") || strings.Contains(said, "apple") {
		t.Fatalf("the agent did not follow the steering message: %q", r.said.String())
	}
}

// TestReopen runs one short turn, stops Devin, starts it again, finds the
// session in Devin's list, reopens it, and checks that the history is
// replayed.
func TestReopen(t *testing.T) {
	a, done := liveApp(t)
	defer done()
	r := record(a)
	s, err := a.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Send(s.ID, "Reply with only the word kiwi.", nil, nil); err != nil {
		t.Fatal(err)
	}
	waitIdle(t, a, s.ID)
	a.Stop()
	r.mu.Lock()
	r.said.Reset()
	r.mu.Unlock()
	list, err := a.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range list {
		t.Logf("listed %s %q %s open=%v", x.ID, x.Title, x.UpdatedAt, x.Open)
		found = found || x.ID == s.ID
	}
	if !found {
		t.Fatalf("session %s is not in Devin's list after a restart", s.ID)
	}
	if _, err := a.OpenSession(s.ID); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !strings.Contains(strings.ToLower(r.said.String()), "kiwi") {
		t.Fatalf("reopening did not replay the history: %q", r.said.String())
	}
	t.Logf("replayed: %q", r.said.String())
}
