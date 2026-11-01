package sessionhost

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
)

func TestCaptureLimitationsStayIdempotentAcrossRestartReplay(t *testing.T) {
	dir := t.TempDir()
	const id = "codex:restart"
	image := acp.ContentBlock{Type: "image", MimeType: "image/png", Data: "aW1hZ2U="}
	tool := acp.Raw(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "binary", "title": "Read artifact", "status": "completed", "content": []any{map[string]any{"type": "content", "content": image}}})
	newCapture := func(replay bool) (*Conversation, *TranscriptCapture) {
		s := NewConversation(Identity{ID: id, Provider: "codex", Cwd: dir}, InitialActivity{}, replay, false)
		var mu sync.Mutex
		c := NewTranscriptCapture(&mu, TranscriptCaptureHost{StateDir: func() string { return dir }, VisibleText: func(text string) string { return text }, RedactText: func(text string) string { return text }, Log: func(string, string, string, ...any) {}, TurnEnd: func(string, string, error) {}})
		c.Initialize(s, replay)
		return s, c
	}
	s, c := newCapture(false)
	c.RecordMessage(s, "you", "Inspect this image.", true, false)
	c.RecordImage(s, image)
	c.RecordUpdate(s, tool)
	c.RecordMessage(s, "agent", "The artifact was inspected.", false, false)
	c.FinishTurn(s, nil, "end_turn")
	seed, err := ReadTranscript(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range seed.Messages {
		if m.Role == "limitation" {
			t.Fatal("ordinary capture created a handoff row")
		}
	}
	// Keep historical generated rows on disk, as an existing installation has
	// them. The projection must hide them without rewriting their text.
	for range 4 {
		seed.Messages = append(seed.Messages, ConversationMessage{Role: "limitation", Text: CaptureOmission, Provider: "codex", SessionID: id})
	}
	if err := WriteTranscript(dir, seed); err != nil {
		t.Fatal(err)
	}
	for restart := range 5 {
		s, c = newCapture(true)
		c.RecordUpdate(s, acp.Raw(map[string]any{"sessionUpdate": "user_message_chunk", "content": acp.ContentBlock{Type: "text", Text: "Inspect this image."}}))
		// A replay that reports an image without returning its bytes must not
		// discard the image already retained in the saved copy.
		c.RecordUpdate(s, acp.Raw(map[string]any{"sessionUpdate": "user_message_chunk", "content": acp.ContentBlock{Type: "image"}}))
		c.RecordUpdate(s, tool)
		c.RecordUpdate(s, acp.Raw(map[string]any{"sessionUpdate": "agent_message_chunk", "content": acp.ContentBlock{Type: "text", Text: "The artifact was inspected."}}))
		c.FinishReplay(s, true)
		got, err := ReadTranscript(dir, id)
		if err != nil || !reflect.DeepEqual(got.Messages, seed.Messages) || !got.Unsupported {
			t.Fatalf("restart %d changed retained history or lost capture metadata: %+v %v", restart, got, err)
		}
		for _, m := range visibleCaptureHistory(got.Messages) {
			if legacyCaptureLimitation(m) {
				t.Fatal("saved view still contains generated capture rows")
			}
		}
	}
}

func TestCaptureLimitationsAreContextualAndDoNotRewriteSavedHistory(t *testing.T) {
	c, s, tr := confirmationFixtureSize(t, 1)
	tr.Unsupported = true
	legacy := ConversationMessage{Role: "limitation", Text: CaptureOmission, Provider: tr.Provider, SessionID: tr.SessionID}
	tr.Messages = append(tr.Messages, legacy, legacy, ConversationMessage{Role: "agent", Text: CaptureOmission})
	if err := WriteTranscript(c.host.StateDir(), tr); err != nil {
		t.Fatal(err)
	}
	file := TranscriptPath(c.host.StateDir(), tr.SessionID)
	before, _ := os.ReadFile(file)
	view := c.Saved(s.ID())
	for _, m := range view.Messages {
		if legacyCaptureLimitation(m) {
			t.Fatal("old generated rows flood the restored view")
		}
	}
	if len(view.Messages) != len(tr.Messages)-2 || view.Warning != "" {
		t.Fatal("projection removed unrelated content or claimed a new failure")
	}
	preview, err := c.PreviewSession(s.ID(), "new")
	if err != nil || countText(preview.Warnings, CaptureOmission) != 1 {
		t.Fatalf("explicit continuation must disclose non-text omission once: %+v %v", preview, err)
	}
	prepared, err := c.Preview(s, "new")
	if err != nil || countText(prepared.Warnings, CaptureOmission) != 1 || !strings.Contains(prepared.Context, CaptureOmission) {
		t.Fatalf("actual handoff lost the omission: %+v %v", prepared, err)
	}
	if countText(preview.Warnings, "Images are not transferred.") != 1 {
		t.Fatal("an unrelated genuine limitation was suppressed")
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(before, after) {
		t.Fatal("reading or previewing rewrote existing history")
	}
	// A limitation recorded by an actual continuation is a different kind and
	// remains part of that continued chat's history after restoration.
	genuine := ConversationMessage{Role: "limitation", Kind: captureHandoffKind, Text: CaptureOmission, Provider: tr.Provider, SessionID: tr.SessionID}
	if got := visibleCaptureHistory([]ConversationMessage{legacy, genuine}); !reflect.DeepEqual(got, []ConversationMessage{genuine}) {
		t.Fatal("a genuine explicit handoff limitation was hidden")
	}
}

func countText(values []string, text string) int {
	n := 0
	for _, value := range values {
		if value == text {
			n++
		}
	}
	return n
}

func TestCaptureLimitationsStayHiddenInParentReplayView(t *testing.T) {
	c, s, tr := confirmationFixtureSize(t, 1)
	tr.ParentSessionID = "prior"
	legacy := ConversationMessage{Role: "limitation", Text: CaptureOmission, Provider: "codex", SessionID: "prior"}
	genuine := ConversationMessage{Role: "limitation", Kind: captureHandoffKind, Text: CaptureOmission, Provider: "codex", SessionID: "prior"}
	quote := ConversationMessage{Role: "agent", Text: CaptureOmission}
	tr.ParentHistory = []ConversationMessage{legacy, legacy, genuine, quote}
	if err := WriteTranscript(c.host.StateDir(), tr); err != nil {
		t.Fatal(err)
	}
	file := TranscriptPath(c.host.StateDir(), tr.SessionID)
	before, _ := os.ReadFile(file)
	for range 3 {
		view, err := c.View(s.ID())
		if err != nil || !reflect.DeepEqual(view.History, []ConversationMessage{genuine, quote}) {
			t.Fatalf("parent replay projection: %+v %v", view, err)
		}
		if saved := c.Saved(s.ID()); len(saved.Messages) != len(tr.Messages)+2 {
			t.Fatalf("saved fallback projection disagrees: %+v", saved)
		}
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(before, after) {
		t.Fatal("view projection changed the saved parent history")
	}
}
