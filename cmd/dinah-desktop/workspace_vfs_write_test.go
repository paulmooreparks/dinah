package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Terminal's writes reach local folders through their virtual paths,
// keep the read revision checks, honour retained identities and leave the
// catalogs alone.
func TestWorkspaceMountWrites(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, root := mountTestVFS(t)
	a := v.service.app
	events := 0
	a.sink = func(name string, _ any) {
		if name == "workspace:changed" {
			events++
		}
	}
	none := ResourceRef{}

	item, err := a.WriteWorkspaceMount("/workspaces/first/local/a.txt", "one\n", WriteCreate, "", none)
	if err != nil || item.Path != "/workspaces/first/local/a.txt" || events != 1 {
		t.Fatalf("create %+v %v (%d events)", item, err, events)
	}
	if _, err := a.WriteWorkspaceMount("/workspaces/first/local/a.txt", "two\n", WriteCreate, "", none); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("create over a file: %v", err)
	}
	read, _ := v.read("/workspaces/first/local/a.txt")
	if _, err := a.WriteWorkspaceMount("/workspaces/first/local/a.txt", "two\n", WriteExpect, read.Revision, none); err != nil {
		t.Fatalf("expected write: %v", err)
	}
	if _, err := a.WriteWorkspaceMount("/workspaces/first/local/a.txt", "lost\n", WriteExpect, read.Revision, none); err == nil {
		t.Fatal("a stale write succeeded")
	}
	if _, err := a.WriteWorkspaceMount("/workspaces/first/local/a.txt", "three\n", WriteReplace, "", none); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(data) != "three\n" {
		t.Fatalf("on disk %q", data)
	}

	if _, err := a.MakeWorkspaceDirectory("/workspaces/first/local/d/e", true, none); err != nil {
		t.Fatalf("mkdir -p: %v", err)
	}
	if _, err := a.CopyWorkspaceMount("/workspaces/first/local/a.txt", none, "/workspaces/first/local/d/e/a.txt", none, false); err != nil {
		t.Fatalf("cp: %v", err)
	}
	if n, err := a.CopyWorkspaceMount("/workspaces/first/local/d", none, "/workspaces/another/files/d", none, true); err != nil || n != 3 {
		t.Fatalf("cp -r between workspaces: %d %v", n, err)
	}
	if _, err := a.MoveWorkspaceMount("/workspaces/first/local/a.txt", none, "/workspaces/first/local/b.txt", none); err != nil {
		t.Fatalf("mv: %v", err)
	}
	if _, err := a.MoveWorkspaceMount("/workspaces/first/local/b.txt", none, "/workspaces/another/files/b.txt", none); err != nil {
		t.Fatalf("mv between folders: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "b.txt")); !os.IsNotExist(err) {
		t.Fatal("the original stayed after a move between folders")
	}
	if n, err := a.RemoveWorkspaceMount("/workspaces/first/local/d", true, none); err != nil || n != 3 {
		t.Fatalf("rm -r: %d %v", n, err)
	}
	if _, err := a.RemoveWorkspaceMount("/workspaces/first/local", true, none); err == nil {
		t.Fatal("a source's own folder was removed")
	}
	if _, err := a.RemoveWorkspaceMount("/workspaces/first", true, none); err == nil {
		t.Fatal("a workspace catalog was removed")
	}
	if _, err := a.RemoveWorkspaceMount("/workspaces/first/local/missing", false, none); err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("a missing file: %v", err)
	}

	// Home is a local folder too.
	if _, err := a.WriteWorkspaceMount("~/notes.txt", "mine\n", WriteCreate, "", none); err != nil {
		t.Fatalf("home write: %v", err)
	}
	// A retained identity keeps a write in its own workspace.
	other := ResourceRef{WorkspaceID: "other", SourceID: "source-two"}
	if _, err := a.WriteWorkspaceMount("/workspaces/first/local/x.txt", "x", WriteCreate, "", other); err == nil {
		t.Fatal("a write went to another workspace under a retained identity")
	}
}
