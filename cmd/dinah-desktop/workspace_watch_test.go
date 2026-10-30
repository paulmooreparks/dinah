package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A folder the page shows is watched on disk: a file written there by another
// program sends one disk-changed event for its source, a burst of writes
// sends one, and a folder no longer shown sends none.
func TestWatchFoldersSendsDiskChanges(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, root := mountTestVFS(t)
	a := v.service.app
	var mu sync.Mutex
	var got []WorkspaceMountChange
	a.sink = func(name string, data any) {
		if change, ok := data.(WorkspaceMountChange); ok && name == "workspace:changed" && change.Kind == "disk-changed" {
			mu.Lock()
			got = append(got, change)
			mu.Unlock()
		}
	}
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(got) }
	wait := func(n int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for count() < n && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
	}
	defer func() {
		if a.folderWatch != nil {
			a.folderWatch.close()
		}
	}()
	workspaceID := a.settings.WorkspaceRecords[0].ID
	ref := ResourceRef{WorkspaceID: workspaceID, SourceID: "source-one", Path: ""}
	if err := a.WatchFolders([]ResourceRef{ref}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(root, "note"+string(rune('a'+i))+".md"), []byte("written elsewhere"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	wait(1)
	time.Sleep(2 * watchQuiet)
	if n := count(); n != 1 || got[0].WorkspaceID != workspaceID || got[0].SourceID != "source-one" {
		t.Fatalf("a burst of writes sent %d events: %+v", n, got)
	}
	if err := a.WatchFolders(nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "later.md"), []byte("not watched"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(4 * watchQuiet)
	if n := count(); n != 1 {
		t.Fatalf("a folder no longer shown sent an event: %+v", got)
	}
}

// A workspace file's path on this computer is in its source's folder.
func TestLocalFilePath(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, root := mountTestVFS(t)
	a := v.service.app
	if err := os.MkdirAll(filepath.Join(root, "meetings"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "meetings", "notes.md"), []byte("notes"), 0600); err != nil {
		t.Fatal(err)
	}
	ref := ResourceRef{WorkspaceID: a.settings.WorkspaceRecords[0].ID, SourceID: "source-one", Path: "meetings/notes.md"}
	path, dir, err := v.localPath(ref)
	if err != nil || dir || path != filepath.Join(root, "meetings", "notes.md") {
		t.Fatalf("path %q %v %v", path, dir, err)
	}
	ref.Path = "meetings"
	if _, dir, err := v.localPath(ref); err != nil || !dir {
		t.Fatalf("a folder was not told as one: %v %v", dir, err)
	}
}
