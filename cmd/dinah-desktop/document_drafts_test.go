package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"studio/internal/drafts"
)

// TestDraftBindings opens a workspace file as a draft, edits and saves it
// through the app's bindings, and checks the events open views rely on.
func TestDraftBindings(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "notes.md"), []byte("# Notes\n"), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(ws)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	events := []string{}
	a.sink = func(name string, data any) { events = append(events, name) }
	ref := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "notes.md"}

	v, err := a.OpenDraft(ref)
	if err != nil {
		t.Fatal(err)
	}
	if v.Text != "# Notes\n" || v.Origin.Kind != "local-folder" || v.Changed || slices.Contains(v.Actions, drafts.ActionSave) {
		t.Fatalf("opened %+v", v)
	}
	again, err := a.OpenDraft(ref)
	if err != nil || again.ID != v.ID {
		t.Fatalf("reopening made another draft: %v", err)
	}
	up, err := a.UpdateDraft(v.ID, v.Revision, "# Notes\n\nSaved from the Editor.\n")
	if err != nil || !slices.Contains(up.Actions, drafts.ActionSave) {
		t.Fatalf("update %+v %v", up.Actions, err)
	}
	if _, err := a.UpdateDraft(v.ID, v.Revision, "lost"); err == nil || !strings.Contains(err.Error(), "another window") {
		t.Fatalf("stale update: %v", err)
	}
	saved, err := a.SaveDraft(up.ID, up.Revision)
	if err != nil || saved.Changed {
		t.Fatalf("save %+v %v", saved, err)
	}
	data, _ := os.ReadFile(filepath.Join(ws, "notes.md"))
	if string(data) != "# Notes\n\nSaved from the Editor.\n" {
		t.Fatalf("file %q", data)
	}
	if err := a.DiscardDraft(saved.ID, saved.Revision); err != nil {
		t.Fatal(err)
	}
	// Each open by the person also puts the document at the top of Recent.
	// The save also tells the page that the document was written where it
	// is kept, so that what draws from it reads it again.
	want := []string{"draft:changed", "documents:recent", "documents:recent", "draft:changed", "draft:changed", "workspace:changed", "draft:changed"}
	if !slices.Equal(events, want) {
		t.Fatalf("events %v", events)
	}
}

// A resource outside every registered workspace cannot be opened.
func TestDraftUnknownWorkspace(t *testing.T) {
	a := NewApp()
	if _, err := a.OpenDraft(ResourceRef{WorkspaceID: "nope", SourceID: "s", Path: "a.md"}); err == nil {
		t.Fatal("opened a draft in an unregistered workspace")
	}
}

func TestEditUnknownExtensionAsText(t *testing.T) {
	isolateWorkspaceSettings(t)
	ws := t.TempDir()
	a := closeBusWhenDone(t, NewApp())
	record := newWorkspaceRecord(ws)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	for _, tc := range []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"sitenav.xfer", []byte("navigation {}\n"), true},
		{"binary.unknown", []byte{0, 1, 2}, false},
		{"encoding.unknown", []byte{0xff, 0xfe, 0x41}, false},
		{"large.unknown", []byte(strings.Repeat("x", maxComputerDocument+1)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(ws, tc.name)
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			ref := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: tc.name}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			item, err := localResourceItem(ref, record.Sources[0], f)
			f.Close()
			if err != nil || !slices.Contains(item.Actions, "edit-text") || slices.Contains(item.Actions, "preview") {
				t.Fatalf("actions %+v: %v", item, err)
			}
			v, err := a.OpenDraft(ref)
			if !tc.valid {
				if err == nil {
					t.Fatal("accepted non-text or oversized file")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			up, err := a.UpdateDraft(v.ID, v.Revision, "navigation { edited }\n")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.SaveDraft(up.ID, up.Revision); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != up.Text {
				t.Fatalf("save did not preserve the original file: %v", err)
			}
		})
	}
}
