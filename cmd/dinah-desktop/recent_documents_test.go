package main

import (
	"os"
	"path/filepath"
	"testing"

	"studio/internal/recent"
)

// The person's opens are put at the top of Documents › Recent with the
// workspace's name as it is now, and an entry that can no longer be opened
// is left out; a document that exists only as a draft is not recorded.
func TestRecentDocumentsRecordsThePersonsOpens(t *testing.T) {
	isolateWorkspaceSettings(t)
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# Plan\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\n"), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(dir)
	record.Name = "Projects"
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	var told []RecentChange
	a.sink = func(name string, data any) {
		if name == "documents:recent" {
			told = append(told, data.(RecentChange))
		}
	}

	if _, err := a.OpenDraft(ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "plan.md"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenComputerFile(record.ID, filepath.Join(dir, "notes.md")); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "loose.txt")
	os.WriteFile(elsewhere, []byte("Loose.\n"), 0o644)
	if _, err := a.OpenComputerFile(record.ID, elsewhere); err != nil {
		t.Fatal(err)
	}
	if _, err := a.NewEmptyDraft(record.ID); err != nil {
		t.Fatal(err)
	}

	got, err := a.RecentDocuments()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || len(told) != 3 {
		t.Fatalf("recent %+v, told %d times", got, len(told))
	}
	if got[0].File != elsewhere || got[0].Kind != recent.KindComputerFile || got[0].WorkspaceName != "Projects" {
		t.Fatalf("newest %+v", got[0])
	}
	if got[1].Ref == nil || got[1].Ref.Path != "notes.md" || got[2].Ref == nil || got[2].Ref.Path != "plan.md" || got[2].WorkspaceName != "Projects" {
		t.Fatalf("older %+v %+v", got[1], got[2])
	}
	for _, change := range told {
		if change.Error != "" {
			t.Fatalf("recording failed: %s", change.Error)
		}
	}

	// A renamed workspace shows its new name; a file that is gone is left out.
	a.settings.WorkspaceRecords[0].Name = "Projects, Renamed"
	os.Remove(elsewhere)
	got, _ = a.RecentDocuments()
	if len(got) != 2 || got[0].WorkspaceName != "Projects, Renamed" {
		t.Fatalf("after rename and removal %+v", got)
	}
	if _, err := os.Stat(filepath.Join(configDir(), recent.FileName)); err != nil {
		t.Fatalf("the list is not in the test's state folder: %v", err)
	}
}
