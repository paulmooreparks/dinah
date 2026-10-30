package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// A replayed history reaches the page in batches, in order, the rest when
// the replay ends; an update after the replay goes alone, as before.
func TestReplayedUpdatesGoInBatches(t *testing.T) {
	a, _ := providerTestApp(t)
	var mu sync.Mutex
	var batches [][]json.RawMessage
	singles := 0
	a.sink = func(name string, data any) {
		mu.Lock()
		defer mu.Unlock()
		switch name {
		case "agent:updates":
			batches = append(batches, data.(map[string]any)["updates"].([]json.RawMessage))
		case "agent:update":
			singles++
		}
	}
	s := newLoadingSessionState(&sessionSeed{ID: "replay-batch", Provider: "codex", Cwd: t.TempDir()}, true, false, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	a.transcriptCapture().Initialize(s, true)
	update := func(i int) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"sessionId": s.ID(), "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": fmt.Sprint(i, " ")}}})
		return b
	}
	for i := 0; i < replayBatch+50; i++ {
		a.newAgentHandler().Notify("session/update", update(i))
	}
	mu.Lock()
	if len(batches) != 1 || len(batches[0]) != replayBatch || singles != 0 {
		t.Fatalf("during the replay: %d batches, %d singles", len(batches), singles)
	}
	mu.Unlock()
	a.flushReplay(s)
	s.SetReplaying(false)
	a.newAgentHandler().Notify("session/update", update(999))
	mu.Lock()
	defer mu.Unlock()
	if len(batches) != 2 || len(batches[1]) != 50 || singles != 1 {
		t.Fatalf("after the replay: %d batches, %d singles", len(batches), singles)
	}
	var last struct {
		Content struct{ Text string } `json:"content"`
	}
	json.Unmarshal(batches[1][49], &last)
	if last.Content.Text != fmt.Sprint(replayBatch+49, " ") {
		t.Fatalf("out of order: %q", last.Content.Text)
	}
}
