package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/applet"
)

func builtinTestContext(ws string) context.Context {
	return applet.WithInvocation(context.Background(), applet.Invocation{Workspace: ws, OwnerKind: "chat", OwnerID: "test", ID: "test"})
}

func TestWorkspaceAppletConfinesAndBoundsReads(t *testing.T) {
	ws := t.TempDir()
	a := NewApp()
	a.settings.WorkspaceRecords = []WorkspaceRecord{newWorkspaceRecord(ws)}
	bundle, err := a.workspaceAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "notes.txt"), []byte("workspace notes"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(ws)
	result, err := r.Execute(ctx, workspaceReadAction, `{"path":"notes.txt"}`)
	if err != nil || !strings.Contains(result.OutputJSON, "workspace notes") {
		t.Fatalf("read: %s, %v", result.OutputJSON, err)
	}
	for _, path := range []string{"../escape", ".devin/mcp_config.json", ".git/config", ".env", "C:/secret", "notes.txt:secret", "a/../notes.txt"} {
		input, _ := json.Marshal(map[string]string{"path": path})
		if _, err := r.Execute(ctx, workspaceReadAction, string(input)); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	if _, err := r.Execute(context.Background(), workspaceReadAction, `{"path":"notes.txt"}`); err == nil {
		t.Fatal("accepted missing invocation")
	}
	if err := os.WriteFile(filepath.Join(ws, "large.txt"), []byte(strings.Repeat("x", (1<<20)+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Execute(ctx, workspaceReadAction, `{"path":"large.txt"}`); err == nil {
		t.Fatal("accepted oversized file")
	}
	if err := os.WriteFile(filepath.Join(ws, "binary.txt"), []byte{0xff, 0x00}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Execute(ctx, workspaceReadAction, `{"path":"binary.txt"}`); err == nil {
		t.Fatal("accepted binary file")
	}
	if err := os.WriteFile(filepath.Join(ws, ".env"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = r.Execute(ctx, workspaceListAction, `{}`)
	if err != nil || strings.Contains(result.OutputJSON, ".env") || !strings.Contains(result.OutputJSON, "notes.txt") {
		t.Fatalf("list: %s, %v", result.OutputJSON, err)
	}
	if err := os.Symlink(filepath.Join(ws, ".env"), filepath.Join(ws, "alias.txt")); err == nil {
		if _, err := r.Execute(ctx, workspaceReadAction, `{"path":"alias.txt"}`); err == nil {
			t.Fatal("accepted symlink to hidden file")
		}
	}
}
