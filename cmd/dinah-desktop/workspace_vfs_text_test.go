package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A document reads a file it refers to by the file's virtual path. A missing
// file is reported as not found, and a folder, a long file and a path
// outside /home and /workspaces are refused.
func TestADocumentReadsAFileItRefersTo(t *testing.T) {
	isolateWorkspaceSettings(t)
	v, root := mountTestVFS(t)
	a := v.service.app
	if err := os.MkdirAll(filepath.Join(root, "drawings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "drawings", "plan.d2"), []byte("a -> b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := a.ReadWorkspaceMountText("/workspaces/first/local/drawings/plan.d2", 0)
	if err != nil || !got.Found || got.Text != "a -> b\n" {
		t.Fatalf("read %+v %v", got, err)
	}
	if got, err := a.ReadWorkspaceMountText("/workspaces/first/local/drawings/plan.d2", 7); err != nil || !got.Found {
		t.Fatalf("a file as long as the limit: %+v %v", got, err)
	}
	if _, err := a.ReadWorkspaceMountText("/workspaces/first/local/drawings/plan.d2", 6); err == nil || !strings.Contains(err.Error(), "longer than 6 bytes") {
		t.Fatalf("a file longer than the limit: %v", err)
	}
	for _, path := range []string{"/workspaces/first/local/drawings/none.d2", "/workspaces/first/local/nowhere/plan.d2"} {
		if got, err := a.ReadWorkspaceMountText(path, 0); err != nil || got.Found || got.Text != "" {
			t.Fatalf("a missing file at %s: %+v %v", path, got, err)
		}
	}
	if _, err := a.ReadWorkspaceMountText("/workspaces/first/local/drawings", 0); err == nil || !strings.Contains(err.Error(), "regular text file") {
		t.Fatalf("a folder: %v", err)
	}
	if _, err := a.ReadWorkspaceMountText("/workspaces/first", 0); err == nil || !strings.Contains(err.Error(), "is not a file") {
		t.Fatalf("a workspace: %v", err)
	}
	for _, path := range []string{"/first/local/drawings/plan.d2", "drawings/plan.d2", "/workspaces/first/local/../local/drawings/plan.d2", "/bin/dinah", "/lib"} {
		if got, err := a.ReadWorkspaceMountText(path, 0); err == nil {
			t.Fatalf("%s was read: %+v", path, got)
		}
	}

	// Home is read the same way, by its full path and by ~.
	if _, err := a.WriteWorkspaceMount("~/shared.css", "p { }\n", WriteCreate, "", ResourceRef{}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"~/shared.css", a.HomeMountPath() + "/shared.css"} {
		if got, err := a.ReadWorkspaceMountText(path, 0); err != nil || got.Text != "p { }\n" {
			t.Fatalf("%s: %+v %v", path, got, err)
		}
	}
}
