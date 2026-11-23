package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Copy Path gives the path of a file or folder on the computer as the
// operating system writes it: absolute, with the system's own separator,
// for a file, a folder and a source's own folder alike. Show in Folder is
// given the same path. A source of another kind has no such path, and
// neither is asked of the system for it.
func TestWorkspaceLocalPathIsNative(t *testing.T) {
	ws := t.TempDir()
	os.MkdirAll(filepath.Join(ws, "Sources", "Inner"), 0o755)
	os.WriteFile(filepath.Join(ws, "Sources", "Inner", "notes.md"), []byte("notes"), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(ws)
	record.Sources = append(record.Sources, WorkspaceSource{ID: "records", Kind: "sample-tracker", Name: "Records"})
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	local := record.Sources[0].ID

	shown := []string{}
	was := showPathInFolder
	showPathInFolder = func(p string) error { shown = append(shown, p); return nil }
	defer func() { showPathInFolder = was }()

	other := "/"
	if os.PathSeparator == '/' {
		other = "\\"
	}
	for _, c := range []struct{ path, want string }{
		{"", ws},
		{"Sources", filepath.Join(ws, "Sources")},
		{"Sources/Inner", filepath.Join(ws, "Sources", "Inner")},
		{"Sources/Inner/notes.md", filepath.Join(ws, "Sources", "Inner", "notes.md")},
	} {
		ref := ResourceRef{WorkspaceID: record.ID, SourceID: local, Path: c.path}
		p, err := a.WorkspaceLocalPath(ref)
		if err != nil || p != c.want {
			t.Fatalf("path of %q: %q %v, want %q", c.path, p, err, c.want)
		}
		if !filepath.IsAbs(p) || strings.Contains(strings.TrimPrefix(p, ws), other) {
			t.Fatalf("path of %q is not in the system's form: %q", c.path, p)
		}
		if err := a.ShowWorkspaceResource(ref); err != nil || shown[len(shown)-1] != c.want {
			t.Fatalf("show %q: %v %v", c.path, shown, err)
		}
	}

	away := ResourceRef{WorkspaceID: record.ID, SourceID: "records", Path: "R-1"}
	if p, err := a.WorkspaceLocalPath(away); err == nil {
		t.Fatalf("a source of another kind gave the path %q", p)
	}
	before := len(shown)
	if err := a.ShowWorkspaceResource(away); err == nil || len(shown) != before {
		t.Fatalf("a source of another kind was shown: %v %v", shown, err)
	}
}

// What has a place on the computer is marked local, whether it is a file, a
// folder or a source's own folder, so the page can offer Copy Path and Show
// in Folder for it. A workspace, the folders above the workspaces and a
// folder that is missing are not marked, and the applets' version 1 actions
// never see the mark.
func TestLocalMarksWhatIsOnTheComputer(t *testing.T) {
	ws := t.TempDir()
	os.MkdirAll(filepath.Join(ws, "Sources"), 0o755)
	os.WriteFile(filepath.Join(ws, "Sources", "notes.md"), []byte("notes"), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(ws)
	record.MountAlias = "alpha"
	record.Sources[0].MountAlias = "documents"
	record.Sources = append(record.Sources, WorkspaceSource{ID: "gone", Kind: "local-folder", Name: "Gone", MountAlias: "gone", Path: filepath.Join(ws, "not-there")})
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}

	for _, p := range []string{"/workspaces/alpha/documents", "/workspaces/alpha/documents/Sources", "/workspaces/alpha/documents/Sources/notes.md"} {
		item, err := a.StatWorkspaceMount(p)
		if err != nil || !item.Local {
			t.Fatalf("%s is on the computer but not marked: %+v %v", p, item, err)
		}
	}
	for _, p := range []string{"/", "/workspaces", "/workspaces/alpha"} {
		item, err := a.StatWorkspaceMount(p)
		if err != nil || item.Local {
			t.Fatalf("%s has no place on the computer but is marked: %+v %v", p, item, err)
		}
	}
	page, err := a.ListWorkspaceMounts("/workspaces/alpha", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	marks := map[string]bool{}
	for _, item := range page.Entries {
		marks[item.Path] = item.Local
	}
	if !marks["/workspaces/alpha/documents"] || marks["/workspaces/alpha/gone"] {
		t.Fatalf("a source's folder is marked by whether it is there: %v", marks)
	}
	listed, err := a.ListWorkspaceMounts("/workspaces/alpha/documents/Sources", 0, 100)
	if err != nil || len(listed.Entries) != 1 || !listed.Entries[0].Local {
		t.Fatalf("a listed file is not marked: %+v %v", listed, err)
	}
	if appletMountItem(listed.Entries[0]).Local {
		t.Fatal("the applets' mount item kept the mark")
	}
	ref := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "Sources/notes.md"}
	resource, err := a.ResolveWorkspaceResourceMount(ref)
	if err != nil || !resource.Local {
		t.Fatalf("a resource found by its reference is not marked: %+v %v", resource, err)
	}
	read, err := a.ReadWorkspaceResource(ref)
	if err != nil || !read.Resource.Local || appletResourceItem(read.Resource).Local {
		t.Fatalf("the resource item's mark: %+v %v", read.Resource, err)
	}
}
