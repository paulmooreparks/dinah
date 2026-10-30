package main

import (
	"os"
	"path/filepath"
	"testing"

	"studio/internal/layout"
)

// Removing a workspace forgets its record and its place in the lists, leaves
// its folder and files alone, and puts another workspace in front when it
// was the one in front.
func TestRemovingAWorkspaceForgetsItAndKeepsItsFiles(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	keep, gone := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(gone, "notes.md"), []byte("kept"), 0o644)
	if _, err := a.SetWorkspace(keep); err != nil {
		t.Fatal(err)
	}
	state, err := a.SetWorkspace(gone)
	if err != nil {
		t.Fatal(err)
	}
	var goneID string
	for _, r := range state.WorkspaceRecords {
		if r.Sources[0].Path == gone || layout.SamePath(r.Sources[0].Path, gone) {
			goneID = r.ID
		}
	}
	if goneID == "" {
		t.Fatal("the workspace was not registered")
	}
	state, err = a.RemoveWorkspace(goneID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range state.WorkspaceRecords {
		if r.ID == goneID {
			t.Fatal("the record is still there")
		}
	}
	for _, p := range state.Recent {
		if layout.SamePath(p, gone) {
			t.Fatal("the folder is still in Recent")
		}
	}
	if !layout.SamePath(state.Workspace, keep) {
		t.Fatalf("the workspace in front is %q, not the other one", state.Workspace)
	}
	if b, err := os.ReadFile(filepath.Join(gone, "notes.md")); err != nil || string(b) != "kept" {
		t.Fatal("a file in the removed workspace's folder was touched")
	}
	if _, err := a.RemoveWorkspace(goneID); err == nil {
		t.Fatal("removing it twice was not refused")
	}
}
