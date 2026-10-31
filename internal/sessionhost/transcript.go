package sessionhost

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

//go:embed transcript.go
var TranscriptSource []byte

// TranscriptPath preserves the conversation file layout for a session ID.
func TranscriptPath(stateDir, id string) string {
	return filepath.Join(stateDir, "conversations", base64.RawURLEncoding.EncodeToString([]byte(id))+".json")
}

// ReadTranscript reads a persisted conversation and verifies its session ID.
func ReadTranscript(stateDir, id string) (*Transcript, error) {
	b, err := os.ReadFile(TranscriptPath(stateDir, id))
	if err != nil {
		return nil, errors.New("the complete chat history is unavailable; reopen the chat to load its history before changing provider")
	}
	var tr Transcript
	if json.Unmarshal(b, &tr) != nil || tr.SessionID != id {
		return nil, errors.New("the saved chat history is invalid")
	}
	return &tr, nil
}

// WriteTranscript replaces the persisted conversation. The caller serializes
// access to the transcript and its file, including the shared temporary path.
func WriteTranscript(stateDir string, tr *Transcript) error {
	b, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	p := TranscriptPath(stateDir, tr.SessionID)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(p+".tmp", b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(p+".tmp", p); err != nil {
		return err
	}
	cacheConfirmation(p, sha256.Sum256(b), confirmationMetadata(tr))
	return nil
}
