package sessionhost

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestTranscriptCaptureSharesSerializationAndTurnEndOrder(t *testing.T) {
	var mu sync.Mutex
	dir := t.TempDir()
	s := NewConversation(Identity{ID: "capture", Provider: "test", Cwd: dir}, InitialActivity{}, false, false)
	var order []string
	locked := func() {
		t.Helper()
		if mu.TryLock() {
			mu.Unlock()
			t.Fatal("capture callback escaped transcript serialization")
		}
	}
	c := NewTranscriptCapture(&mu, TranscriptCaptureHost{
		StateDir: func() string { locked(); order = append(order, "path"); return dir },
		VisibleText: func(text string) string {
			locked()
			order = append(order, "visible")
			return strings.ReplaceAll(text, "secret", "redacted")
		},
		RedactText: func(text string) string {
			locked()
			order = append(order, "redact")
			return strings.ReplaceAll(text, "secret", "redacted")
		},
		TurnEnd: func(id, reason string, err error) {
			if !mu.TryLock() {
				t.Fatal("turn log called under transcript lock")
			}
			defer mu.Unlock()
			if id != "capture" || reason != "failed" || err == nil {
				t.Fatal("turn log arguments changed")
			}
			tr := s.HistorySavedLocked()
			if len(tr.Messages) != 2 || !strings.Contains(tr.Messages[1].Text, "redacted") {
				t.Fatalf("status missing before turn log: %#v", tr)
			}
			order = append(order, "log")
		},
	})
	if c.mu != &mu {
		t.Fatal("capture replaced the host mutex")
	}
	c.Initialize(s, false)
	order = nil
	entries := []ConversationMessage{{Role: "delivered", Text: "secret", Delivered: &DeliveredMessage{}}}
	c.RecordDelivered(s, "secret prompt", entries)
	if entries[0].Text != "secret" {
		t.Fatal("delivery redaction mutated caller input")
	}
	c.FinishTurn(s, errors.New("secret"), "failed")
	if want := []string{"visible", "redact", "visible", "log", "path"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("effect order %v, want %v", order, want)
	}
	tr, err := ReadTranscript(dir, s.ID())
	if err != nil || !tr.Complete || strings.Contains(tr.Messages[0].Text, "secret") {
		t.Fatalf("final capture: %#v, %v", tr, err)
	}
}

func TestTranscriptCaptureSettlesHeldReplayBeforeUnlockedFlush(t *testing.T) {
	var mu sync.Mutex
	s := NewConversation(Identity{ID: "replay"}, InitialActivity{}, true, false)
	flushes := 0
	c := NewTranscriptCapture(&mu, TranscriptCaptureHost{ReplayBatch: 1, FlushReplay: func(got *Conversation) {
		if !mu.TryLock() {
			t.Fatal("flush called under transcript mutex")
		}
		mu.Unlock()
		if got != s || s.State().HoldingReplay {
			t.Fatal("flush preceded held replay settlement")
		}
		if batch := s.DrainReplay(); len(batch) != 1 {
			t.Fatalf("flush batch %+v", batch)
		}
		flushes++
	}})
	c.ReplayBoundary(s, "user_message_chunk")
	update := json.RawMessage(`{"sessionUpdate":"user_message_chunk","content":{"type":"text","text":"hello"}}`)
	if !s.HoldReplay(update) {
		t.Fatal("user replay was not held")
	}
	c.ReplayBoundary(s, "agent_message_chunk")
	c.SettleReplay(s)
	if flushes != 1 {
		t.Fatalf("flush count %d", flushes)
	}
}

func TestTranscriptCaptureTombstoneLookupAndSavedRepresentation(t *testing.T) {
	var mu sync.Mutex
	dir := t.TempDir()
	s := NewConversation(Identity{ID: "deleted", Provider: "test", Cwd: dir}, InitialActivity{}, false, false)
	initial := &Transcript{SessionID: s.ID(), Provider: "test", Workspace: dir, ParentSessionID: "parent", Complete: true, PendingHandoff: true, Handoff: "context", Summary: "summary", CaptureError: "error", Unsupported: true, Messages: []ConversationMessage{{Text: "private"}}, ParentHistory: []ConversationMessage{{Text: "parent"}}}
	if err := WriteTranscript(dir, initial); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	s.HistoryRestoreSavedLocked(initial)
	mu.Unlock()
	lookups, paths := 0, 0
	c := NewTranscriptCapture(&mu, TranscriptCaptureHost{
		Session: func(id string) *Conversation {
			if !mu.TryLock() {
				t.Fatal("lookup called under transcript mutex")
			}
			mu.Unlock()
			lookups++
			return s
		},
		StateDir: func() string {
			if mu.TryLock() {
				mu.Unlock()
				t.Fatal("path lookup outside transcript mutex")
			}
			paths++
			return dir
		},
	})
	if err := c.Tombstone(s.ID()); err != nil {
		t.Fatal(err)
	}
	tr, err := ReadTranscript(dir, s.ID())
	if err != nil {
		t.Fatal(err)
	}
	if !tr.Deleted || !tr.Complete || tr.ParentSessionID != "parent" || tr.Messages != nil || tr.ParentHistory != nil || tr.PendingHandoff || tr.Unsupported || tr.Handoff != "" || tr.Summary != "" || tr.CaptureError != "" {
		t.Fatalf("tombstone representation %#v", tr)
	}
	if !reflect.DeepEqual(tr, s.HistorySavedLocked()) {
		t.Fatal("live history differs from saved tombstone")
	}
	if lookups != 1 || paths != 2 {
		t.Fatalf("lookup counts %d, %d", lookups, paths)
	}
	if err := c.Tombstone("missing"); err != nil {
		t.Fatalf("missing transcript: %v", err)
	}
}
