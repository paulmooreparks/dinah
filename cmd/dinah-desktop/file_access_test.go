package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"studio/internal/applet"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/fileaccess"
)

func TestFileAccessCachedServiceReadDoesNotTakeAppLock(t *testing.T) {
	a, _ := providerTestApp(t)
	service, err := a.fileAccessService()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ordinary.txt")
	if err := os.WriteFile(path, []byte("ordinary content"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := fileaccess.Scope{Chat: "cached-reader", Workspace: dir, Roots: []string{dir}, Layout: stateLayout()}
	done := make(chan error, 1)
	a.mu.Lock()
	go func() {
		cached, err := a.fileAccessService()
		if err == nil && cached != service {
			err = fmt.Errorf("the cached service changed")
		}
		if err == nil {
			var text string
			text, err = cached.Read(context.Background(), path, nil, nil, scope)
			if err == nil && text != "ordinary content" {
				err = fmt.Errorf("read returned %q", text)
			}
		}
		done <- err
	}()
	select {
	case err = <-done:
		a.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		a.mu.Unlock()
		// Let a blocked accessor finish before the fixture closes its store.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		t.Fatal("a cached service read waited on the app lock")
	}
}

func TestFileAccessBackgroundNotificationReachesSettings(t *testing.T) {
	a, _ := providerTestApp(t)
	if _, err := a.fileAccessService(); err != nil {
		t.Fatal(err)
	}
	a.fileAccessUpdated("file-access:read", nil)
	events := make(chan string, 8)
	a.sink = func(name string, _ any) { events <- name }
	changes := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	released := make(chan struct{})
	a.watchFileAccessChanges(ctx, changes, func() { close(released) })
	// The core notification guard tests real revision changes. Here the host
	// fixture has not yet observed the revision carried by the notification.
	a.fileAccess.mu.Lock()
	a.fileAccess.networkState = ""
	a.fileAccess.mu.Unlock()
	changes <- struct{}{}
	select {
	case name := <-events:
		if name != "file-access:network" {
			t.Fatal("wrong Settings event", name)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Settings did not hear completion before the audit poll")
	}
	changes <- struct{}{}
	select {
	case name := <-events:
		t.Fatal("duplicate revision emitted another event", name)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("network subscription was not released")
	}
}

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

func newFileAccessFixtureApp(t *testing.T) *App {
	t.Helper()
	// Establish the state directory before NewApp binds its stores to it.
	// Taking no App argument makes the former post-construction use a
	// compile error instead of silently redirecting only some stores.
	isolateWorkspaceSettings(t)
	a := NewApp()
	home := t.TempDir()
	a.fileAccess.resolveDefaults = func() ([]fileaccess.Rule, error) {
		return fileaccess.DefaultRules(runtime.GOOS, home, filepath.Join(home, "roaming"), filepath.Join(home, "local"), nil), nil
	}
	t.Cleanup(a.closeFileAccess)
	t.Cleanup(a.closeChatStates)
	return a
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
	ref.Path = ""
	if page, err := a.listWorkspaceResources(fileAccessCallerContext(t, a, root), ref, 0, 100); err != nil || len(page.Entries) != 0 {
		t.Fatalf("agent listing disclosed denied metadata: %+v %v", page, err)
	}
	if page, err := a.ListWorkspaceResources(ref, 0, 100); err != nil || len(page.Entries) != 1 {
		t.Fatalf("person listing authority changed: %+v %v", page, err)
	}
	if a.fileAccessRefused("test") != 2 {
		t.Fatal("metadata refusal not attributed to calling chat")
	}
}

func TestAgentWorkspaceStatUsesPolicyAndAudit(t *testing.T) {
	a, _ := providerTestApp(t)
	root := t.TempDir()
	a.settings.Workspace = root
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "workspace-a", MountAlias: "workspace", ExecutionPath: root, PrimarySourceID: "source", Sources: []WorkspaceSource{{ID: "source", MountAlias: "files", Kind: "local-folder", Name: "Files", Path: root}}}}
	path := filepath.Join(root, "plain.txt")
	if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	policy, err := a.FileAccessPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddFileAccessEntry(path, policy.Version); err != nil {
		t.Fatal(err)
	}
	ref := ResourceRef{WorkspaceID: "workspace-a", SourceID: "source", Path: "plain.txt"}
	service := a.resourceService()
	service.ctx = fileAccessCallerContext(t, a, root)
	if item, err := (workspaceVFS{service: service, allowedWorkspace: "workspace-a"}).statRef(ref); err == nil || item.Size != 0 {
		t.Fatalf("denied stat returned metadata: %+v %v", item, err)
	}
	if _, err := a.ResolveWorkspaceResourceMount(ref); err != nil {
		t.Fatal("person stat authority changed", err)
	}
	log, err := a.FileAccessLog(fileaccess.Filter{Chat: "test"})
	if err != nil || len(log.Rows) == 0 || log.Rows[0].Source != workspaceMountStatAction {
		t.Fatalf("stat source attribution: %+v %v", log, err)
	}
	if a.fileAccessRefused("test") != 1 {
		t.Fatal("stat refusal was not audited")
	}
}
