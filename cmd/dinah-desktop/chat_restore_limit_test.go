package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

// heldSettings makes the test agent keep every request to change a setting
// unanswered until the returned function is called, which a cleanup also
// does. Each request signals entered before waiting, and answered once it
// has been let go, just before the agent answers it.
func heldSettings(t *testing.T, c *testProviderConnection) (release func(), entered, answered chan struct{}) {
	t.Helper()
	gate := make(chan struct{})
	answered = make(chan struct{}, 8)
	entered = make(chan struct{}, 8)
	var once sync.Once
	release = func() { once.Do(func() { close(gate) }) }
	c.onSetOption = func(string) {
		entered <- struct{}{}
		<-gate
		answered <- struct{}{}
	}
	t.Cleanup(release)
	return release, entered, answered
}

func requireSettingsReplyStillHeld(t *testing.T, entered, answered chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-t.Context().Done():
		t.Fatal("the restore request did not reach the held provider")
	}
	select {
	case <-answered:
		t.Fatal("the chat opened only after the provider's restore reply")
	default:
	}
}

// chatOptionValue is the current value of one of a chat's settings.
func chatOptionValue(a *App, chat, option string) string {
	for _, o := range a.SessionOptions(chat) {
		if o.ID == option {
			return string(o.CurrentValue)
		}
	}
	return ""
}

func savedOption(a *App, chat, option string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return string(a.settings.SessionOptions[chat][option])
}

// A reopened chat whose agent answers the load and then never answers the
// request to restore a setting opens and becomes usable before that reply. It has
// the agent's values, it says so once, it keeps its saved choices, and no
// second request is sent to the agent that is not answering. The answer
// that comes later changes nothing.
func TestAReopenedChatOpensWhenARestoreIsNeverAnswered(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchEvents(a)
	c.options = append(c.options, acp.ConfigOption{ID: "effort", Name: "Effort", Type: "select", CurrentValue: json.RawMessage(`"low"`),
		Options: json.RawMessage(`[{"value":"low","name":"Low"},{"value":"high","name":"High"}]`)})
	release, entered, answered := heldSettings(t, c)
	a.optionService().RestoreLimit = 40 * time.Millisecond
	ws := t.TempDir()
	const id = "codex:unrestored"
	a.settings.SessionOptions = map[string]map[string]json.RawMessage{id: {"model": json.RawMessage(`"changed"`), "effort": json.RawMessage(`"high"`)}}

	info, err := a.OpenSessionAt(ws, id)
	if err != nil {
		t.Fatalf("the open failed because a setting was not restored: %v", err)
	}
	requireSettingsReplyStillHeld(t, entered, answered)
	s := a.session(id)
	if s == nil || !s.Open() {
		t.Fatal("the chat is not open")
	}
	if state := s.State(); !state.Ready || state.Restoring || state.Replaying {
		t.Fatalf("the chat is not usable: %+v", state)
	}
	if len(info.Notes) != 1 || !strings.Contains(info.Notes[0], "did not answer") || !strings.Contains(info.Notes[0], "model") || !strings.Contains(info.Notes[0], "Effort") {
		t.Fatalf("the chat does not say once which settings were left: %q", info.Notes)
	}
	if n := countCalls(c, "session/set_config_option"); n != 1 {
		t.Fatalf("the agent was sent %d requests to change a setting, and it did not answer the first", n)
	}
	if model, effort := chatOptionValue(a, id, "model"), chatOptionValue(a, id, "effort"); model != `"initial"` || effort != `"low"` {
		t.Fatalf("the chat shows model %s and effort %s, and the agent has initial and low", model, effort)
	}
	if model, effort := savedOption(a, id, "model"), savedOption(a, id, "effort"); model != `"changed"` || effort != `"high"` {
		t.Fatalf("the chat's saved choices became model %s and effort %s", model, effort)
	}

	// The late answer reaches nobody. The events counted are those of the
	// chat's options, which an answer to this request would send. The
	// chat's information is not counted, because the open itself still
	// sends it once more after it has returned, with or without an answer.
	options := events.count(sessionhost.EventOptions)
	release()
	select {
	case <-answered:
	case <-time.After(testWait):
		t.Fatal("the held request was not let go")
	}
	time.Sleep(100 * time.Millisecond)
	if got := events.count(sessionhost.EventOptions); got != options {
		t.Fatalf("the late answer sent the page the chat's options: %d events before it and %d after", options, got)
	}
	if model := chatOptionValue(a, id, "model"); model != `"initial"` {
		t.Fatalf("the late answer changed the chat's model to %s", model)
	}
	if model := savedOption(a, id, "model"); model != `"changed"` {
		t.Fatalf("the late answer changed the chat's saved model to %s", model)
	}
	if s := a.session(id); s == nil || !s.Open() {
		t.Fatal("the late answer closed the chat")
	}
}

// A new chat whose agent never answers the request for the person's default
// setting is made and usable before that reply, with the agent's own value.
func TestANewChatOpensWhenARestoreIsNeverAnswered(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchEvents(a)
	release, entered, answered := heldSettings(t, c)
	a.optionService().RestoreLimit = 40 * time.Millisecond
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"codex": {"model": json.RawMessage(`"codex-default"`)}}

	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatalf("the chat was not made because a default was not restored: %v", err)
	}
	requireSettingsReplyStillHeld(t, entered, answered)
	s := a.session(info.ID)
	if s == nil || !s.Open() {
		t.Fatal("the new chat is not open")
	}
	if state := s.State(); !state.Ready || state.Restoring {
		t.Fatalf("the new chat is not usable: %+v", state)
	}
	if model := chatOptionValue(a, info.ID, "model"); model != `"initial"` {
		t.Fatalf("the new chat shows model %s, and the agent has initial", model)
	}
	// The page has no place for a note on a chat it has just made, so the
	// line is in the chat's log.
	a.chatLogs().Close(info.ID)
	noted, err := os.ReadFile(a.chatLogs().Path(info.ID))
	if err != nil || strings.Count(string(noted), "did not answer the request to restore") != 1 {
		t.Fatalf("the unrestored default is not in the chat's log once: %q %v", noted, err)
	}

	options := events.count(sessionhost.EventOptions)
	release()
	select {
	case <-answered:
	case <-time.After(testWait):
		t.Fatal("the held request was not let go")
	}
	time.Sleep(100 * time.Millisecond)
	if events.count(sessionhost.EventOptions) != options {
		t.Fatal("the late answer changed what the page is told of the chat")
	}
	if model := chatOptionValue(a, info.ID, "model"); model != `"initial"` {
		t.Fatalf("the late answer changed the new chat's model to %s", model)
	}
}
