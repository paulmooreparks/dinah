package ollamaagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/acp"
)

func TestEmptyConversationSurvivesRestartBeforeFirstTurn(t *testing.T) {
	srv, seen := fakeOllama(t, "read_file")
	state, workspace := t.TempDir(), t.TempDir()
	ctx := context.Background()
	c, err := Start(srv.URL, state, &host{})
	if err != nil {
		t.Fatal(err)
	}
	var created acp.NewSessionResult
	if err := c.Call(ctx, "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: workspace}}, &created); err != nil {
		t.Fatal(err)
	}
	c.Close()
	restarted, err := Start(srv.URL, state, &host{})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	var listed acp.ListSessionsResult
	if err := restarted.Call(ctx, "session/list", acp.ListSessionsParams{Cwd: workspace}, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Sessions) != 1 || listed.Sessions[0].SessionID != created.SessionID {
		t.Fatalf("empty chat lost after restart: %+v", listed)
	}
	if err := restarted.Call(ctx, "session/load", SessionParams{LoadSessionParams: acp.LoadSessionParams{SessionID: created.SessionID, Cwd: workspace}}, nil); err != nil {
		t.Fatal(err)
	}
	if len(*seen) != 0 {
		t.Fatalf("creation or restoration sent %d model requests", len(*seen))
	}
}

func TestEmptyConversationIsRefusedWhenItsRecordCannotBeSaved(t *testing.T) {
	srv, _ := fakeOllama(t, "read_file")
	c, err := Start(srv.URL, t.TempDir(), &host{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	c.dir = filepath.Join(blocked, "sessions")
	var created acp.NewSessionResult
	err = c.Call(context.Background(), "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: t.TempDir()}}, &created)
	if err == nil || created.SessionID != "" || len(c.sessions) != 0 {
		t.Fatalf("unsaved chat was published: %+v, %v", created, err)
	}
}
