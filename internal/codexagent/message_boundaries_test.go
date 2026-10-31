package codexagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

type boundaryCaptureHandler struct {
	session *sessionhost.Conversation
	capture *sessionhost.TranscriptCapture
	events  []json.RawMessage
}

func (h *boundaryCaptureHandler) Notify(_ string, raw json.RawMessage) {
	var p struct {
		Update json.RawMessage `json:"update"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	h.events = append(h.events, append(json.RawMessage{}, raw...))
	h.capture.RecordUpdate(h.session, p.Update)
}
func (*boundaryCaptureHandler) Request(context.Context, string, json.RawMessage) (any, *acp.Error) {
	return nil, nil
}

// This crosses the real adapter and saved capture with identity-labelled input.
func TestInterlacedAssistantItemsRetainIdentityAndCompletion(t *testing.T) {
	var mu sync.Mutex
	dir := t.TempDir()
	s := sessionhost.NewConversation(sessionhost.Identity{ID: "codex:fixture", Provider: "codex", Cwd: dir}, sessionhost.InitialActivity{}, false, false)
	capture := sessionhost.NewTranscriptCapture(&mu, sessionhost.TranscriptCaptureHost{StateDir: func() string { return dir }, VisibleText: func(s string) string { return s }, RedactText: func(s string) string { return s }})
	capture.Initialize(s, false)
	h := &boundaryCaptureHandler{session: s, capture: capture}
	c := newConn(h)
	c.sessions[s.ID()] = &session{streamed: map[string]bool{}}
	notify := func(method string, p any) { c.notification(method, acp.Raw(p)) }
	notify("turn/started", map[string]any{"threadId": "fixture", "turn": turn{ID: "turn-one"}})
	notify("item/agentMessage/delta", map[string]string{"threadId": "fixture", "turnId": "turn-one", "itemId": "old-item", "delta": "Earlier partial"})
	capture.RecordDelivered(s, "notice", []sessionhost.ConversationMessage{{Role: "delivered", Text: "team notice", Delivered: &sessionhost.DeliveredMessage{ID: 1, Prompt: "notice"}}})
	notify("item/agentMessage/delta", map[string]string{"threadId": "fixture", "turnId": "turn-one", "itemId": "old-item", "delta": " continued"})
	notify("item/agentMessage/delta", map[string]string{"threadId": "fixture", "turnId": "turn-one", "itemId": "new-item", "delta": "New commentary"})
	notify("item/completed", map[string]any{"threadId": "fixture", "turnId": "turn-one", "item": map[string]string{"id": "new-item", "type": "agentMessage", "text": "New commentary complete"}})
	notify("item/completed", map[string]any{"threadId": "fixture", "turnId": "turn-one", "item": map[string]string{"id": "old-item", "type": "agentMessage", "text": "Earlier partial continued"}})
	saved := s.HistorySavedLocked()
	var agents []string
	for _, m := range saved.Messages {
		if m.Role == "agent" {
			agents = append(agents, m.Text)
		}
	}
	report := map[string]any{"events": h.events, "messages": saved.Messages, "agents": agents}
	if out := os.Getenv("DINAH_BOUNDARY_PROBE_OUTPUT"); out != "" {
		b, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(filepath.Clean(out), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(agents, []string{"Earlier partial continued", "New commentary complete"}) {
		t.Fatalf("item separation/completion lost: %#v", agents)
	}
	// Provider replay supplies full items with their original enclosing turn.
	// It must produce the same identified rows and keep the notice independent.
	replayDir := t.TempDir()
	replayed := sessionhost.NewConversation(sessionhost.Identity{ID: s.ID(), Provider: "codex", Cwd: replayDir}, sessionhost.InitialActivity{}, false, false)
	replayCapture := sessionhost.NewTranscriptCapture(&mu, sessionhost.TranscriptCaptureHost{StateDir: func() string { return replayDir }, VisibleText: func(s string) string { return s }, RedactText: func(s string) string { return s }})
	replayCapture.Initialize(replayed, false)
	replayAdapter := newConn(&boundaryCaptureHandler{session: replayed, capture: replayCapture})
	replayAdapter.itemInTurn(s.ID(), "turn-one", acp.Raw(map[string]string{"id": "old-item", "type": "agentMessage", "text": "Earlier partial continued"}), true, true)
	replayCapture.RecordDelivered(replayed, "notice", []sessionhost.ConversationMessage{{Role: "delivered", Text: "team notice", Delivered: &sessionhost.DeliveredMessage{ID: 1, Prompt: "notice"}}})
	replayAdapter.itemInTurn(s.ID(), "turn-one", acp.Raw(map[string]string{"id": "new-item", "type": "agentMessage", "text": "New commentary complete"}), true, true)
	if !reflect.DeepEqual(saved.Messages, replayed.HistorySavedLocked().Messages) {
		t.Fatalf("live/replay boundary mismatch: live=%+v replay=%+v", saved.Messages, replayed.HistorySavedLocked().Messages)
	}
}
