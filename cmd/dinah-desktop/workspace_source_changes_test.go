package main

import (
	"slices"
	"sync"
	"testing"
	"time"

	"studio/internal/workspaces"
)

// A kind's word that a source changed where it is kept reaches the page as
// one source-changed event for each workspace the source is in, however many
// times the kind says it while it reads, and nothing is sent for a source no
// workspace has or after the window stops hearing.
func TestASourceChangedWhereItIsKeptIsSentToThePageOnce(t *testing.T) {
	registerFakeWiki()
	source := WorkspaceSource{ID: "wiki", Kind: "test-wiki", Name: "Customer Wiki", Extra: workspaces.WithExtra(nil, "pageId", "100")}
	service, ref := resourceTestService(t, source)
	a := service.app
	var mu sync.Mutex
	var got []WorkspaceMountChange
	a.sink = func(name string, data any) {
		if change, ok := data.(WorkspaceMountChange); ok && name == "workspace:changed" && change.Kind == "source-changed" {
			mu.Lock()
			got = append(got, change)
			mu.Unlock()
		}
	}
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(got) }
	if workspaces.SourceChangesHeard() {
		t.Fatal("changes are heard before the window starts")
	}
	a.hearSourceChanges()
	defer a.stopSourceChanges()
	if !workspaces.SourceChangesHeard() {
		t.Fatal("the window does not hear changes")
	}
	for i := 0; i < 5; i++ {
		workspaces.SourceChanged("wiki")
	}
	workspaces.SourceChanged("in-no-workspace")
	deadline := time.Now().Add(testWait)
	for count() < 1 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(2 * watchQuiet)
	if n := count(); n != 1 || got[0].WorkspaceID != ref.WorkspaceID || got[0].SourceID != "wiki" {
		t.Fatalf("a burst of changes sent %d events: %+v", n, got)
	}
	a.stopSourceChanges()
	if workspaces.SourceChangesHeard() {
		t.Fatal("changes are still heard after the window stopped")
	}
	workspaces.SourceChanged("wiki")
	time.Sleep(2 * watchQuiet)
	if n := count(); n != 1 {
		t.Fatalf("a change after the window stopped was sent: %+v", got)
	}
}

// Refresh has a source's provider forget what it kept about the place shown,
// and does nothing for a folder on the computer, which keeps nothing.
func TestRefreshHasAProviderForgetWhatItKept(t *testing.T) {
	registerFakeWiki()
	source := WorkspaceSource{ID: "wiki", Kind: "test-wiki", Name: "Customer Wiki", Extra: workspaces.WithExtra(nil, "pageId", "100")}
	service, ref := resourceTestService(t, source)
	a := service.app
	a.settings.WorkspaceRecords[0].Sources = append(a.settings.WorkspaceRecords[0].Sources, WorkspaceSource{ID: "local", Kind: "local-folder", Name: "Local", Path: t.TempDir()})
	fakeWikiForgotMu.Lock()
	fakeWikiForgot = nil
	fakeWikiForgotMu.Unlock()

	if err := a.RefreshWorkspaceResource(ref); err != nil {
		t.Fatal(err)
	}
	if err := a.RefreshWorkspaceResource(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "wiki", Path: "200"}); err != nil {
		t.Fatal(err)
	}
	if err := a.RefreshWorkspaceResource(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "local"}); err != nil {
		t.Fatalf("a local folder refused to refresh: %v", err)
	}
	fakeWikiForgotMu.Lock()
	forgot := slices.Clone(fakeWikiForgot)
	fakeWikiForgotMu.Unlock()
	if !slices.Equal(forgot, []string{".", "200"}) {
		t.Fatalf("the provider was asked to forget %v", forgot)
	}
	if err := a.RefreshWorkspaceResource(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "nobody"}); err == nil {
		t.Fatal("a source the workspace does not have was refreshed")
	}
}
