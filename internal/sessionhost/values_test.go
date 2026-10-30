package sessionhost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"studio/internal/acp"
)

func TestValueJSONPreservesNilAndOmittedFields(t *testing.T) {
	subagent := `{"id":"","parentId":"","rootId":"","name":"","task":"","state":"","canCancel":false}`
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"message", ConversationMessage{}, `{"role":"","text":"","provider":"","sessionId":""}`},
		{"empty optional message images", ConversationMessage{Images: []acp.ContentBlock{}}, `{"role":"","text":"","provider":"","sessionId":""}`},
		{"delivery", DeliveredMessage{}, `{"id":0,"kind":"","from":"","time":""}`},
		{"attachment", Attachment{}, `{"kind":"","name":""}`},
		{"document", DocRef{}, `{"path":"","title":""}`},
		{"subagent", SubagentInfo{}, subagent},
		{"nil subagent updates", SubagentRecord{}, `{"subagent":` + subagent + `,"updates":null,"seq":0}`},
		{"empty subagent updates", SubagentRecord{Updates: []json.RawMessage{}}, `{"subagent":` + subagent + `,"updates":[],"seq":0}`},
		{"nil transcript messages", Transcript{}, `{"sessionId":"","provider":"","workspace":"","complete":false,"messages":null}`},
		{"empty transcript messages", Transcript{Messages: []ConversationMessage{}, ParentHistory: []ConversationMessage{}}, `{"sessionId":"","provider":"","workspace":"","complete":false,"messages":[]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := json.Marshal(c.value)
			if err != nil || string(got) != c.want {
				t.Fatalf("JSON = %s, error = %v; want %s", got, err, c.want)
			}
		})
	}
}

func TestTranscriptStorageKeepsLayoutAndRepresentation(t *testing.T) {
	root := t.TempDir()
	for _, messages := range [][]ConversationMessage{nil, {}, {{Role: "agent", Text: "answer", Delivered: &DeliveredMessage{ID: 7, Kind: "question", Prompt: "original"}}}} {
		tr := &Transcript{SessionID: "codex:chat", Provider: "codex", Workspace: "workspace", Messages: messages}
		if err := WriteTranscript(root, tr); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "conversations", "Y29kZXg6Y2hhdA.json")
		data, err := os.ReadFile(path)
		want, _ := json.Marshal(tr)
		if err != nil || string(data) != string(want) {
			t.Fatalf("stored %s, error %v; want %s", data, err, want)
		}
		read, err := ReadTranscript(root, tr.SessionID)
		if err != nil || !reflect.DeepEqual(read, tr) {
			t.Fatalf("read %+v, error %v; want %+v", read, err, tr)
		}
	}
	if _, err := os.Stat(TranscriptPath(root, "codex:chat") + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains: %v", err)
	}
}

func TestTranscriptReadErrorsKeepTheirMessages(t *testing.T) {
	root := t.TempDir()
	if _, err := ReadTranscript(root, "missing"); err == nil || err.Error() != "the complete chat history is unavailable; reopen the chat to load its history before changing provider" {
		t.Fatalf("missing history: %v", err)
	}
	if err := WriteTranscript(root, &Transcript{SessionID: "chat"}); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{`, `{"sessionId":"other"}`} {
		if err := os.WriteFile(TranscriptPath(root, "chat"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadTranscript(root, "chat"); err == nil || err.Error() != "the saved chat history is invalid" {
			t.Fatalf("invalid history: %v", err)
		}
	}
}
