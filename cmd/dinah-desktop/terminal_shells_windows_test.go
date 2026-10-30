//go:build windows

package main

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// A Terminal runs a real shell in the workspace's folder: what is typed
// reaches it, what it draws arrives as events, and its end is reported.
func TestATerminalRunsARealShellInTheWorkspace(t *testing.T) {
	a, _ := providerTestApp(t)
	folder := t.TempDir()
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "w1", Name: "Parser", ExecutionPath: folder}}
	a.settings.WorkspaceID = "w1"
	var mu sync.Mutex
	var drawn strings.Builder
	exited := make(chan int, 1)
	a.sink = func(name string, data any) {
		switch name {
		case "terminal:output":
			b, _ := base64.StdEncoding.DecodeString(data.(map[string]string)["data"])
			mu.Lock()
			drawn.Write(b)
			mu.Unlock()
		case "terminal:exit":
			exited <- data.(map[string]any)["code"].(int)
		}
	}
	started, err := a.TerminalStart("cmd", nil, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	if a.ExitStatus().Terminals != 1 {
		t.Fatal("running terminal missing from exit status")
	}
	a.exitPrompt = func(context.Context, exitStatus) bool { return false }
	if a.confirmExitIfNeeded(nil) || a.ExitStatus().Terminals != 1 {
		t.Fatal("cancelling exit interrupted the terminal")
	}
	if started.Folder != folder || started.Name != "Command Prompt" {
		t.Fatalf("the shell started as %+v", started)
	}
	if err := a.TerminalResize(started.ID, 120, 40); err != nil {
		t.Fatal(err)
	}
	if err := a.TerminalWrite(started.ID, "echo shell in %CD%\r\nexit 7\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-exited:
		if code != 7 {
			t.Fatalf("the shell ended with %d", code)
		}
	case <-time.After(20 * time.Second):
		a.terminalService().CloseAll()
		t.Fatal("the shell did not end")
	}
	if a.ExitStatus().Terminals != 0 {
		t.Fatal("ended terminal remained in exit status")
	}
	mu.Lock()
	out := drawn.String()
	mu.Unlock()
	if !strings.Contains(out, "shell in "+folder) {
		t.Fatalf("the shell drew %q", out)
	}
	if err := a.TerminalWrite(started.ID, "dir\r\n"); err == nil {
		t.Fatal("an ended shell took input")
	}
}

func TestTheShellsListEndsWithDinahsOwn(t *testing.T) {
	a, _ := providerTestApp(t)
	got := a.TerminalShells()
	if len(got.Shells) < 2 || got.Shells[len(got.Shells)-1].ID != dinahShell || got.Default != got.Shells[0].ID {
		t.Fatalf("the shells are %+v", got)
	}
	if err := a.SetTerminalShell("cmd"); err != nil {
		t.Fatal(err)
	}
	if a.TerminalShells().Default != "cmd" {
		t.Fatal("the chosen shell is not the default")
	}
	if err := a.SetTerminalShell("no-such-shell"); err == nil {
		t.Fatal("a shell that is not here was chosen")
	}
	if folderWithin(os.TempDir(), "../..") != "" {
		t.Fatal("a folder outside the source was used")
	}
}
