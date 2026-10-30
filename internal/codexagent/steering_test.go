package codexagent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestSteeringTargetsTheActiveTurnWithoutInterrupting(t *testing.T) {
	var calls int
	c := fake(t, nil, func(m message, send func(message)) {
		calls++
		if m.Method != "turn/steer" {
			t.Errorf("steering sent %s", m.Method)
		}
		var p struct {
			ThreadID string           `json:"threadId"`
			TurnID   string           `json:"expectedTurnId"`
			Input    []map[string]any `json:"input"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			t.Error(err)
		}
		if p.ThreadID != "thread" || p.TurnID != "active" || len(p.Input) != 2 || p.Input[0]["text"] != "Keep the scope small." || p.Input[1]["type"] != "image" {
			t.Errorf("steer input: %+v", p)
		}
		send(answer(m, map[string]string{"turnId": "active"}))
	})
	c.sessions["codex:thread"] = &session{turn: "active", completion: make(chan turn, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	delivered := 0
	err := c.Steer(ctx, acp.PromptParams{SessionID: "codex:thread", Prompt: []acp.ContentBlock{{Type: "text", Text: "Keep the scope small."}, {Type: "image", MimeType: "image/png", Data: "YWJj"}}}, func() { delivered++ })
	if err != nil || delivered != 1 || calls != 1 {
		t.Fatalf("err=%v delivered=%d calls=%d", err, delivered, calls)
	}
	c.mu.Lock()
	c.sessions["codex:thread"].completion = nil
	c.mu.Unlock()
	if err := c.Steer(ctx, acp.PromptParams{SessionID: "codex:thread"}, nil); !errors.Is(err, acp.ErrNoActiveTurn) {
		t.Fatalf("idle steering: %v", err)
	}
}

func TestFailedNativeSteeringDoesNotAcknowledgeDelivery(t *testing.T) {
	c := fake(t, nil, func(m message, send func(message)) {
		send(message{ID: m.ID, Error: &acp.Error{Code: -32600, Message: "turn already ended"}})
	})
	c.sessions["codex:thread"] = &session{turn: "active", completion: make(chan turn, 1)}
	delivered := false
	err := c.Steer(context.Background(), acp.PromptParams{SessionID: "codex:thread", Prompt: []acp.ContentBlock{{Type: "text", Text: "Change direction."}}}, func() { delivered = true })
	if err == nil || delivered {
		t.Fatalf("err=%v delivered=%v", err, delivered)
	}
}

func TestSteeringChecksCancellationAndImageCapability(t *testing.T) {
	c := fake(t, nil, func(m message, send func(message)) { t.Error("cancelled turn issued RPC") })
	c.sessions["codex:thread"] = &session{model: "text-only", turn: "active", completion: make(chan turn, 1), cancelRequested: true}
	c.models = []model{{Model: "text-only", InputModalities: []string{"text"}}, {Model: "vision", InputModalities: []string{"text", "image"}}}
	if !c.CanSteerContent("codex:thread", []string{"text"}) || c.CanSteerContent("codex:thread", []string{"image"}) {
		t.Fatal("text-only capability mismatch")
	}
	c.sessions["codex:thread"].model = "vision"
	if !c.CanSteerContent("codex:thread", []string{"image"}) {
		t.Fatal("vision capability missing")
	}
	if err := c.Steer(context.Background(), acp.PromptParams{SessionID: "codex:thread"}, nil); !errors.Is(err, acp.ErrNoActiveTurn) {
		t.Fatalf("cancelled turn: %v", err)
	}
}
