package sessionhost

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
)

func identifiedFixture(t *testing.T) (*Conversation, *TranscriptCapture, string) {
	t.Helper()
	dir := t.TempDir()
	s := NewConversation(Identity{ID: "codex:messages", Provider: "codex", Cwd: dir}, InitialActivity{}, false, false)
	var mu sync.Mutex
	c := NewTranscriptCapture(&mu, TranscriptCaptureHost{StateDir: func() string { return dir }, VisibleText: func(text string) string {
		for _, secret := range []string{"private-token", "dinah-applet-v1:SYNTHETIC.NOTAUTH"} {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
		return text
	}, RedactText: func(s string) string { return s }, TurnEnd: func(string, string, error) {}})
	c.Initialize(s, false)
	return s, c, dir
}
func identifiedUpdate(c *TranscriptCapture, s *Conversation, turn, item, mode, text string) {
	c.RecordUpdate(s, acp.Raw(map[string]any{"sessionUpdate": "agent_message_chunk", "turnId": turn, "itemId": item, "contentMode": mode, "content": map[string]string{"type": "text", "text": text}}))
}

func TestIdentifiedCaptureKeepsItemsAcrossNoticeAndLegacyBoundaries(t *testing.T) {
	s, c, dir := identifiedFixture(t)
	identifiedUpdate(c, s, "turn", "old", "append", "Earlier")
	c.RecordDelivered(s, "notice", []ConversationMessage{{Role: "delivered", Text: "notice", Delivered: &DeliveredMessage{ID: 1}}})
	identifiedUpdate(c, s, "turn", "old", "append", " continued")
	identifiedUpdate(c, s, "turn", "new", "append", "New")
	c.RecordUpdate(s, acp.Raw(map[string]any{"sessionUpdate": "agent_thought_chunk", "turnId": "turn", "itemId": "new", "contentMode": "snapshot", "content": map[string]string{"type": "text", "text": "hidden thought"}}))
	c.RecordUpdate(s, json.RawMessage(`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":" legacy"}}`))
	identifiedUpdate(c, s, "turn", "new", "snapshot", "New complete")
	identifiedUpdate(c, s, "turn", "new", "append", " rejected late suffix")
	identifiedUpdate(c, s, "turn", "new", "snapshot", "New complete")
	identifiedUpdate(c, s, "another-turn", "new", "snapshot", "Other turn")
	c.FinishTurn(s, nil, "end_turn")
	tr, err := ReadTranscript(dir, s.ID())
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, m := range tr.Messages {
		if m.Role == "agent" {
			texts = append(texts, m.Text)
		}
	}
	if !reflect.DeepEqual(texts, []string{"Earlier continued", "New complete", " legacy", "Other turn"}) {
		t.Fatalf("rows %#v", tr.Messages)
	}
	if tr.Messages[0].ItemID != "old" || tr.Messages[2].ItemID != "new" || !tr.Messages[2].MessageComplete {
		t.Fatal("identity/completion not persisted")
	}
	identifiedUpdate(c, s, "turn", "old", "append", " after turn")
	if got := s.HistorySavedLocked().Messages[0].Text; got != "Earlier continued" {
		t.Fatalf("retired append revived row: %q", got)
	}
}

func TestIdentifiedSnapshotsIncludeEmptyAndRedactReconstructedText(t *testing.T) {
	for _, secret := range []string{"private-token", "dinah-applet-v1:SYNTHETIC.NOTAUTH"} {
		for cut := 1; cut < len(secret); cut++ {
			s, c, dir := identifiedFixture(t)
			identifiedUpdate(c, s, "turn", "item", "append", secret[:cut])
			identifiedUpdate(c, s, "turn", "item", "append", secret[cut:])
			if got := s.HistorySavedLocked().Messages[0].Text; got != "[redacted]" {
				t.Fatalf("split %d leaked %q", cut, got)
			}
			c.FinishTurn(s, nil, "end_turn")
			persisted, err := ReadTranscript(dir, s.ID())
			if err != nil || persisted.Messages[0].Text != "[redacted]" {
				t.Fatalf("split %d persisted unredacted text: %+v %v", cut, persisted, err)
			}
			identifiedUpdate(c, s, "turn", "item", "snapshot", "private-token complete")
			if got := s.HistorySavedLocked().Messages[0].Text; got != "[redacted] complete" {
				t.Fatal(got)
			}
			identifiedUpdate(c, s, "turn", "item", "snapshot", "")
			identifiedUpdate(c, s, "turn", "item", "append", "resurrection")
			m := s.HistorySavedLocked().Messages[0]
			if m.Text != "" || !m.MessageComplete {
				t.Fatalf("empty snapshot %+v", m)
			}
		}
	}
}

func TestIdentifiedIndexSurvivesToolRowInsertion(t *testing.T) {
	s, c, _ := identifiedFixture(t)
	s.HistoryRecordToolLocked(false, s.ID(), "tool", true, "tool", "in_progress", nil, false)
	identifiedUpdate(c, s, "turn", "item", "append", "message")
	s.HistoryRecordToolLocked(false, s.ID(), "tool", false, "", "completed", []string{"result"}, false)
	identifiedUpdate(c, s, "turn", "item", "snapshot", "complete message")
	var agents []string
	for _, m := range s.HistorySavedLocked().Messages {
		if m.Role == "agent" {
			agents = append(agents, m.Text)
		}
	}
	if !reflect.DeepEqual(agents, []string{"complete message"}) {
		t.Fatalf("shifted row identity %v", agents)
	}
}

func TestIdentifiedCaptureRetiresAcrossRestoreReplayAndDeletion(t *testing.T) {
	s, c, dir := identifiedFixture(t)
	identifiedUpdate(c, s, "turn", "item", "append", "unfinished")
	s.HistoryRestoreSavedLocked(s.HistorySavedLocked())
	identifiedUpdate(c, s, "turn", "item", "append", " late")
	if s.HistorySavedLocked().Messages[0].Text != "unfinished" {
		t.Fatal("restore revived a partial accumulator")
	}
	c.FinishTurn(s, nil, "end_turn")
	reopened := NewConversation(Identity{ID: s.ID(), Provider: "codex", Cwd: dir}, InitialActivity{}, true, false)
	c.Initialize(reopened, true)
	identifiedUpdate(c, reopened, "turn", "item", "snapshot", "unfinished")
	c.FinishReplay(reopened, true)
	identifiedUpdate(c, reopened, "turn", "item", "append", " late")
	if reopened.HistorySavedLocked().Messages[0].Text != "unfinished" {
		t.Fatal("replay retained an active accumulator")
	}
	s.HistoryReplaceDeletedLocked(&Transcript{SessionID: s.ID(), Provider: "codex", Workspace: dir, Deleted: true})
	identifiedUpdate(c, s, "other", "new", "snapshot", "must not reappear")
	if len(s.HistorySavedLocked().Messages) != 0 {
		t.Fatal("late item revived removed history")
	}
}
