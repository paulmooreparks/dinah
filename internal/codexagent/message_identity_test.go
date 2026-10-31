package codexagent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestIdentifiedUpdatesKeepOriginalReplayTurnAndEmptySnapshots(t *testing.T) {
	h := &testHandler{}
	c := fake(t, h, func(m message, send func(message)) {
		switch m.Method {
		case "thread/read":
			send(answer(m, map[string]any{"thread": thread{ID: "thread", Cwd: "C:/fixture"}}))
		case "model/list":
			send(answer(m, catalogResult()))
		case "thread/resume":
			send(answer(m, threadResult{Thread: thread{ID: "thread", Turns: []turn{{ID: "original", Items: []json.RawMessage{acp.Raw(map[string]string{"id": "item", "type": "agentMessage", "text": ""})}}}}}))
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Call(ctx, "session/load", acp.LoadSessionParams{SessionID: "codex:thread", Cwd: "C:/fixture"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(h.events) != 1 {
		t.Fatalf("empty replay snapshot missing: %d", len(h.events))
	}
	var event struct {
		Update struct {
			acp.MessageIdentity
			Content acp.ContentBlock
		} `json:"update"`
	}
	if err := json.Unmarshal(h.events[0], &event); err != nil {
		t.Fatal(err)
	}
	if event.Update.TurnID != "original" || event.Update.ItemID != "item" || event.Update.ContentMode != "snapshot" || event.Update.Content.Text != "" {
		t.Fatalf("replay identity %+v", event)
	}
}

func TestAdapterRejectsRetiredTurnAndRemovedSessionDeltas(t *testing.T) {
	h := &testHandler{}
	c := newConn(h)
	c.sessions["codex:thread"] = &session{streamed: map[string]bool{}}
	notify := func(method string, p any) { c.notification(method, acp.Raw(p)) }
	notify("turn/started", map[string]any{"threadId": "thread", "turn": turn{ID: "old"}})
	notify("item/agentMessage/delta", map[string]string{"threadId": "thread", "turnId": "old", "itemId": "item", "delta": "accepted"})
	notify("turn/completed", map[string]any{"threadId": "thread", "turn": turn{ID: "old"}})
	notify("item/agentMessage/delta", map[string]string{"threadId": "thread", "turnId": "old", "itemId": "late", "delta": "rejected"})
	notify("turn/started", map[string]any{"threadId": "thread", "turn": turn{ID: "new"}})
	notify("item/agentMessage/delta", map[string]string{"threadId": "thread", "turnId": "old", "itemId": "item", "delta": "rejected"})
	notify("item/reasoning/summaryTextDelta", map[string]string{"threadId": "thread", "turnId": "new", "itemId": "thought", "delta": "reasoning"})
	delete(c.sessions, "codex:thread")
	notify("item/agentMessage/delta", map[string]string{"threadId": "thread", "turnId": "new", "itemId": "item", "delta": "rejected"})
	notify("item/completed", map[string]any{"threadId": "thread", "turnId": "new", "item": map[string]string{"id": "item", "type": "agentMessage", "text": "rejected"}})
	if len(h.events) != 2 {
		t.Fatalf("retired updates escaped: %s", h.events)
	}
	var event struct {
		Update struct {
			Kind string `json:"sessionUpdate"`
			acp.MessageIdentity
		} `json:"update"`
	}
	_ = json.Unmarshal(h.events[1], &event)
	if event.Update.Kind != "agent_thought_chunk" || event.Update.TurnID != "new" || event.Update.ContentMode != "append" {
		t.Fatalf("reasoning envelope %+v", event)
	}
}
