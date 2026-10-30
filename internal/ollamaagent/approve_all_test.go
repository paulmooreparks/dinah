package ollamaagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/layout"
)

func TestApproveAllRunsCommandsWithoutAsking(t *testing.T) {
	srv := fakeCommands(t, "echo approved > approved.txt", "dinah credential get")
	h := &host{answer: "reject"}
	c, err := Start(srv.URL, t.TempDir(), h)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ws := t.TempDir()
	var opened acp.NewSessionResult
	if err := c.Call(context.Background(), "session/new", SessionParams{LoadSessionParams: acp.LoadSessionParams{Cwd: ws}}, &opened); err != nil {
		t.Fatal(err)
	}
	c.SetApprovals("all")
	for _, text := range []string{"run", "reserved"} {
		if err := c.Call(context.Background(), "session/prompt", acp.PromptParams{SessionID: opened.SessionID, Prompt: []acp.ContentBlock{{Type: "text", Text: text}}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(ws, "approved.txt")); err != nil {
		t.Fatal(err)
	}
	if len(h.asked) != 0 {
		t.Fatal("Approve Everything asked for command permission")
	}
	if !strings.Contains(h.said(), "does not let an agent") {
		t.Fatal("reserved command was not refused")
	}
	if strings.Contains(system(ws, true, "all"), "person is asked") {
		t.Fatal("approval warning remained in system prompt")
	}
	b, _ := json.Marshal(ownTools("all"))
	if strings.Contains(string(b), "asked first") || strings.Contains(string(b), "asks the person") {
		t.Fatal("approval warning remained in tool descriptions")
	}
	c.SetApprovals("ask")
	c.mu.Lock()
	s := c.sessions[opened.SessionID]
	commands, applets := s.commands, s.approvals
	c.mu.Unlock()
	if commands != "ask" || applets != "ask" {
		t.Fatal("open chat did not restore asking")
	}
}

type writeHost struct {
	host
	state layout.Layout
}

func (h *writeHost) Request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	if method != "fs/write_text_file" {
		return h.host.Request(ctx, method, params)
	}
	var p acp.WriteTextFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
	}
	if err := agents.WriteTextFile(h.state, p.Path, p.Content); err != nil {
		return nil, &acp.Error{Code: acp.CodeInternal, Message: err.Error()}
	}
	return struct{}{}, nil
}

func TestApproveAllOutsideWritesKeepProtectedSettings(t *testing.T) {
	h := &writeHost{state: layout.Layout{State: t.TempDir()}}
	c := &Conn{handler: h}
	tt := turnTools{c: c, cwd: t.TempDir(), commands: ApprovalsAll}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if result, ok := tt.write(context.Background(), "write", outside, "approved"); !ok {
		t.Fatal(result)
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "approved" {
		t.Fatalf("outside write: %s %v", b, err)
	}
	protected := filepath.Join(h.state.State, "settings.json")
	if _, ok := tt.write(context.Background(), "protected", protected, "{}"); ok {
		t.Fatal("protected settings were written")
	}
	if _, err := os.Stat(protected); !os.IsNotExist(err) {
		t.Fatal("protected settings file was created")
	}
	if len(h.asked) != 0 {
		t.Fatal("Approve Everything asked before an outside write")
	}
}
