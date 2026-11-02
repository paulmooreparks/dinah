package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"studio/internal/acp"
	"testing"
)

func TestProviderFileReadRequiresLiveOwnedSession(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	s := newSessionState(&sessionSeed{ID: "codex:reader", Provider: "codex", Cwd: ws}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	path := filepath.Join(t.TempDir(), "sibling.txt")
	if err := os.WriteFile(path, []byte("allowed"), 0600); err != nil {
		t.Fatal(err)
	}
	h := a.newAgentHandler()
	request := func(id string) (any, *acp.Error) {
		p, _ := json.Marshal(acp.ReadTextFileParams{SessionID: id, Path: path})
		return h.Request(context.Background(), "fs/read_text_file", p)
	}
	if _, err := request(s.ID()); err == nil {
		t.Fatal("unbound caller read")
	}
	h.BindConnection(c)
	if _, err := request("unknown"); err == nil {
		t.Fatal("fake session read")
	}
	if res, err := request(s.ID()); err != nil || res.(acp.ReadTextFileResult).Content != "allowed" {
		t.Fatalf("owned session read: %v %v", res, err)
	}
	other := newSessionState(&sessionSeed{ID: "claude:other", Provider: "claude", Cwd: t.TempDir()}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(other.ID(), other)
	if _, err := request(other.ID()); err == nil {
		t.Fatal("cross-provider session read")
	}
	closed := newSessionState(&sessionSeed{ID: "codex:closed", Provider: "codex", Cwd: ws}, sessionInitial{})
	a.runtime.PutConversationLocked(closed.ID(), closed)
	if _, err := request(closed.ID()); err == nil {
		t.Fatal("closed session read")
	}
	negative := -5
	p, _ := json.Marshal(acp.ReadTextFileParams{SessionID: s.ID(), Path: path, Limit: &negative})
	if _, err := h.Request(context.Background(), "fs/read_text_file", p); err == nil {
		t.Fatal("negative range accepted at request boundary")
	}
	replacement := newTestProviderConnection()
	a.runtime.PutProviderLocked("codex", newTestProvider(replacement, acp.InitializeResult{}))
	if _, err := request(s.ID()); err == nil {
		t.Fatal("retired provider generation read")
	}
	replacement.Close()
	a.runtime.PutProviderLocked("codex", newTestProvider(c, acp.InitializeResult{}))
	c.Close()
	if _, err := request(s.ID()); err == nil {
		t.Fatal("ended connection read")
	}
}
