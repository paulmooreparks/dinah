package main

import (
	"os"
	"path/filepath"
	"strconv"
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
		deadline := time.Now().Add(testWait)
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

// A file a document refers to is in a folder no tree shows. Reading it for
// the document watches its folder, so writing the file again sends a
// disk-changed event for its source, the folders the page shows changing
// does not stop that, and only the most recently read folders are watched.
func TestAFileReadForADocumentIsWatched(t *testing.T) {
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
		deadline := time.Now().Add(testWait)
		for count() < n && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if count() != n {
			t.Fatalf("%d events, want %d: %+v", count(), n, got)
		}
	}
	t.Cleanup(func() {
		if a.folderWatch != nil {
			a.folderWatch.close()
		}
	})
	drawing := filepath.Join(root, "drawings", "plan.d2")
	if err := os.MkdirAll(filepath.Dir(drawing), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(drawing, []byte("a -> b"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := a.ReadWorkspaceMountText("/workspaces/first/local/drawings/plan.d2", 0); err != nil || !got.Found {
		t.Fatalf("read %+v %v", got, err)
	}
	if err := os.WriteFile(drawing, []byte("a -> c"), 0600); err != nil {
		t.Fatal(err)
	}
	wait(1)
	if got[0].WorkspaceID != a.settings.WorkspaceRecords[0].ID || got[0].SourceID != "source-one" {
		t.Fatalf("the event is for another source: %+v", got[0])
	}

	// The page showing other folders, or none, leaves the read folder watched.
	if err := a.WatchFolders(nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * watchQuiet)
	if err := os.WriteFile(drawing, []byte("a -> d"), 0600); err != nil {
		t.Fatal(err)
	}
	wait(2)

	// A file that is not there yet is watched for as well.
	if got, err := a.ReadWorkspaceMountText("/workspaces/first/local/drawings/later.d2", 0); err != nil || got.Found {
		t.Fatalf("a missing file: %+v %v", got, err)
	}
	time.Sleep(2 * watchQuiet)
	if err := os.WriteFile(filepath.Join(root, "drawings", "later.d2"), []byte("x -> y"), 0600); err != nil {
		t.Fatal(err)
	}
	wait(3)

	// More folders read than the limit: the first one read is let go.
	for i := 0; i < readFolderLimit; i++ {
		dir := filepath.Join(root, "set", "d"+strconv.Itoa(i))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := a.ReadWorkspaceMountText("/workspaces/first/local/set/d"+strconv.Itoa(i)+"/none.d2", 0); err != nil {
			t.Fatal(err)
		}
	}
	a.folderWatch.mu.Lock()
	watched, _, held := len(a.folderWatch.dirs), len(a.folderWatch.reads), a.folderWatch.dirs[filepath.Dir(drawing)]
	a.folderWatch.mu.Unlock()
	if watched != readFolderLimit || held.SourceID != "" {
		t.Fatalf("%d folders are watched and the first is %+v", watched, held)
	}
	time.Sleep(2 * watchQuiet)
	before := count()
	if err := os.WriteFile(drawing, []byte("a -> e"), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(4 * watchQuiet)
	if count() != before {
		t.Fatalf("a folder that was let go sent an event: %+v", got)
	}
}
