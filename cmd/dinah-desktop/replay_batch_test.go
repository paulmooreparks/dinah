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
	// As Load Full History opens a chat: the page draws the replay.
	s.SetReplayToPage(true)
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

// An ordinary reopening draws the saved copy and the journal, so its replayed
// updates are numbered in the journal, where the page reads them, and none
// is sent to the page, which would only have built them into a log it then
// discarded (9 November 2026).
func TestAnOrdinaryReopeningSendsThePageNoReplay(t *testing.T) {
	a, _ := providerTestApp(t)
	var mu sync.Mutex
	sent := 0
	a.sink = func(name string, data any) {
		mu.Lock()
		defer mu.Unlock()
		if name == "agent:updates" || name == "agent:update" {
			sent++
		}
	}
	s := newLoadingSessionState(&sessionSeed{ID: "replay-quiet", Provider: "codex", Cwd: t.TempDir()}, true, false, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	a.transcriptCapture().Initialize(s, true)
	update := func(i int) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"sessionId": s.ID(), "update": map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]any{"type": "text", "text": fmt.Sprint(i, " ")}}})
		return b
	}
	for i := 0; i < replayBatch+5; i++ {
		a.newAgentHandler().Notify("session/update", update(i))
	}
	a.flushReplay(s)
	mu.Lock()
	if sent != 0 {
		t.Fatalf("%d replay events reached the page", sent)
	}
	mu.Unlock()
	if left := s.DrainReplay(); len(left) != 0 {
		t.Fatalf("%d replayed updates stayed queued", len(left))
	}
	gen, head := s.StreamPosition()
	if view := s.ChatEvents(gen, 0); head < uint64(replayBatch+5) || len(view.Events) < replayBatch+5 {
		t.Fatalf("the journal has %d events to head %d; the page could not read the replay back", len(view.Events), head)
	}
	s.SetReplaying(false)
	a.newAgentHandler().Notify("session/update", update(999))
	mu.Lock()
	defer mu.Unlock()
	if sent != 1 {
		t.Fatalf("a live update after the replay was sent %d times", sent)
	}
}
