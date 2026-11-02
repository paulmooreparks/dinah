package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"studio/internal/applet"
	"testing"

	"studio/internal/acp"
	"studio/internal/fileaccess"
)

func TestFileAccessPageAndProviderShareOnePolicy(t *testing.T) {
	a, c := providerTestApp(t)
	cwd := t.TempDir()
	a.settings.Workspace = cwd
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "workspace-a", ExecutionPath: cwd}}
	session := newSessionState(&sessionSeed{ID: "codex:policy-reader", Provider: "codex", Cwd: cwd}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(session.ID(), session)
	outside := t.TempDir()
	path := filepath.Join(outside, "plain.txt")
	if err := os.WriteFile(path, []byte("synthetic text"), 0600); err != nil {
		t.Fatal(err)
	}
	policy, err := a.FileAccessPolicy()
	if err != nil {
		t.Fatal(err)
	}
	check, err := a.CheckFileAccessPath(path, "workspace-a")
	if err != nil || !check.Valid || check.InsideWorkspace {
		t.Fatalf("preview: %+v %v", check, err)
	}
	if _, err = a.CheckFileAccessPath(path, "unknown"); err == nil {
		t.Fatal("preview accepted unknown workspace")
	}
	policy, err = a.AddFileAccessEntry(outside, policy.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.SetFileAccessDefault("protected-state", false, policy.Version); err == nil {
		t.Fatal("page disabled protected rule")
	}
	if _, err = a.RemoveFileAccessEntry("protected-state", policy.Version); err == nil {
		t.Fatal("page removed protected rule")
	}
	if text, err := a.readAgentTextFile(context.Background(), c, acp.ReadTextFileParams{SessionID: session.ID(), Path: path}); err == nil || text != "" {
		t.Fatal("provider bypassed page policy")
	}
	if a.fileAccessRefused(session.ID()) != 1 {
		t.Fatal("refusal cache not updated")
	}
	state := a.execOf(context.Background(), nil, "", session.ID(), session.State())
	if state.RefusedReads != 1 || state.State != execIdle {
		t.Fatalf("passive count changed execution state: %+v", state)
	}
	page, err := a.FileAccessLog(fileaccess.Filter{Chat: session.ID()})
	if err != nil || len(page.Rows) != 1 || page.Rows[0].Outcome != "denied" {
		t.Fatalf("log: %+v %v", page, err)
	}
	if _, err := a.readAgentTextFile(context.Background(), c, acp.ReadTextFileParams{SessionID: session.ID(), Path: path}); err == nil {
		t.Fatal("second denied read allowed")
	}
	count, err := a.AcknowledgeFileAccessLog(session.ID(), page.HighWater)
	if err != nil || count != 1 || a.fileAccessRefused(session.ID()) != 1 {
		t.Fatal("ack discarded newer refusal", err)
	}
	if _, err := a.AcknowledgeFileAccessLog(session.ID(), page.HighWater+100); err == nil {
		t.Fatal("future acknowledgement accepted")
	}
	page, err = a.FileAccessLog(fileaccess.Filter{Chat: session.ID()})
	if err != nil {
		t.Fatal(err)
	}
	count, err = a.AcknowledgeFileAccessLog(session.ID(), page.HighWater)
	if err != nil || count != 0 || a.fileAccessRefused(session.ID()) != 0 {
		t.Fatal("displayed log did not clear count", err)
	}
}

func configureFileAccessFixture(t *testing.T, a *App) {
	t.Helper()
	home := t.TempDir()
	a.fileAccess.resolveDefaults = func() ([]fileaccess.Rule, error) {
		return fileaccess.DefaultRules(runtime.GOOS, home, filepath.Join(home, "roaming"), filepath.Join(home, "local"), nil), nil
	}
	t.Cleanup(a.closeFileAccess)
	t.Cleanup(a.closeChatStates)
}
func fileAccessCallerContext(t *testing.T, a *App, workspace string) context.Context {
	t.Helper()
	a.runtime.PutConversationLocked("test", newSessionState(&sessionSeed{ID: "test", Cwd: workspace}, sessionInitial{Open: true}))
	return applet.WithInvocation(context.Background(), applet.Invocation{Workspace: workspace, OwnerKind: "chat", OwnerID: "test", ID: "test"})
}

func TestAgentSourceReadUsesPolicyWhilePersonPreviewKeepsItsAuthority(t *testing.T) {
	a, _ := providerTestApp(t)
	root := t.TempDir()
	a.settings.Workspace = root
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "workspace-a", ExecutionPath: root, PrimarySourceID: "source", Sources: []WorkspaceSource{{ID: "source", Kind: "local-folder", Name: "Files", Path: root}}}}
	path := filepath.Join(root, "plain.txt")
	if err := os.WriteFile(path, []byte("synthetic source"), 0600); err != nil {
		t.Fatal(err)
	}
	policy, err := a.FileAccessPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.AddFileAccessEntry(path, policy.Version); err != nil {
		t.Fatal(err)
	}
	ref := ResourceRef{WorkspaceID: "workspace-a", SourceID: "source", Path: "plain.txt"}
	if read, err := a.readWorkspaceResource(fileAccessCallerContext(t, a, root), ref); err == nil || read.Text != "" {
		t.Fatal("agent source read bypassed policy")
	}
	if read, err := a.ReadWorkspaceResource(ref); err != nil || read.Text != "synthetic source" {
		t.Fatal("person's preview authority changed", err)
	}
	if a.fileAccessRefused("test") != 1 {
		t.Fatal("source refusal not attributed to calling chat")
	}
}
