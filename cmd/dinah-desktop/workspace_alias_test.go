package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"studio/internal/workspaces"
)

func TestWorkspaceMountAliasMigrationAndRename(t *testing.T) {
	isolateWorkspaceSettings(t)
	first, second := newWorkspaceRecord(t.TempDir()), newWorkspaceRecord(t.TempDir())
	first.ID, second.ID = "ws_a", "ws_b"
	first.Name, second.Name = "Design Documents", "Design Documents"
	first.MountAlias, second.MountAlias = "old-work", "design-documents"
	first.Sources[0].Name = "Laptop Notes"
	first.Sources = append(first.Sources, WorkspaceSource{ID: "zz_more", Kind: "local-folder", Name: "Laptop Notes", Path: t.TempDir(), MountAlias: "laptop-notes"})
	if err := saveSettings(Settings{WorkspaceRecords: []WorkspaceRecord{first, second}}); err != nil {
		t.Fatal(err)
	}
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	records := a.State().WorkspaceRecords
	if records[0].MountAlias != "Design Documents" || records[1].MountAlias != "Design Documents (2)" || records[0].Sources[0].MountAlias != "Laptop Notes" {
		t.Fatalf("current labels were not migrated: %+v", records)
	}
	if _, err := a.RenameWorkspace(first.ID, "New Display Label"); err != nil {
		t.Fatal(err)
	}
	restarted := closeBusWhenDone(t, NewApp())
	if restarted.startupErr != nil {
		t.Fatal(restarted.startupErr)
	}
	if got := restarted.State().WorkspaceRecords[0]; got.MountAlias != "New Display Label" || !reflect.DeepEqual(got.Sources, records[0].Sources) {
		t.Fatal("rename did not update its public path")
	}
	attached, err := restarted.AttachLocalWorkspaceSource(first.ID, t.TempDir(), "Laptop Notes")
	if err != nil {
		t.Fatal(err)
	}
	aliases := map[string]bool{}
	for _, source := range attached.Sources {
		aliases[source.MountAlias] = true
	}
	if !aliases["Laptop Notes"] || !aliases["Laptop Notes (2)"] || !aliases["Laptop Notes (3)"] {
		t.Fatalf("attachment collision: %+v", attached)
	}
}

func TestWorkspaceMountNamesAreDeterministicAndReplaceSavedAliases(t *testing.T) {
	first := WorkspaceRecord{ID: "a", Name: "Same Name", MountAlias: "../old", Sources: []WorkspaceSource{{ID: "s2", Name: "Same"}, {ID: "s1", Name: "Same"}}}
	second := WorkspaceRecord{ID: "b", Name: "Same Name", MountAlias: "old"}
	forward := []WorkspaceRecord{first, second}
	reverse := []WorkspaceRecord{second, first}
	if err := ensureWorkspaceMountAliases(forward); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorkspaceMountAliases(reverse); err != nil {
		t.Fatal(err)
	}
	if forward[0].MountAlias != reverse[1].MountAlias || forward[1].MountAlias != reverse[0].MountAlias || forward[0].Sources[0].MountAlias != "Same (2)" || forward[0].Sources[1].MountAlias != "Same" {
		t.Fatalf("unstable namespace: %+v %+v", forward, reverse)
	}
	for _, name := range []string{"中文", "..", "Two  Words", "Many/Separators\\And%Escapes", "ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ"} {
		alias := workspaces.MountAlias(name, "Workspace", map[string]bool{})
		if alias == "" || alias == "." || alias == ".." || strings.ContainsAny(alias, "/\\\x00") {
			t.Fatalf("invalid generated path segment %q", alias)
		}
	}
}

func TestWorkspaceMountRenameRebasesReferencesWithoutAliasRedirect(t *testing.T) {
	isolateWorkspaceSettings(t)
	root := t.TempDir()
	a := closeBusWhenDone(t, NewApp())
	state, err := a.SetWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	record := state.WorkspaceRecords[0]
	source := record.Sources[0]
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	ref := ResourceRef{WorkspaceID: record.ID, SourceID: source.ID, Path: "notes.txt"}
	before, err := a.ResolveWorkspaceResourceMount(ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenameWorkspace(record.ID, "Local"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenameWorkspaceSource(record.ID, source.ID, "Archive"); err != nil {
		t.Fatal(err)
	}
	current, err := a.ResolveWorkspaceResourceMount(ref)
	if err != nil || current.Path != "/workspaces/Local/Archive/notes.txt" {
		t.Fatalf("current path %+v %v", current, err)
	}
	if _, err := a.StatWorkspaceMountExpected(before.Path, ref); err == nil {
		t.Fatal("old public path still resolves")
	}
	catalog, err := a.ResolveWorkspaceResourceMount(ResourceRef{WorkspaceID: record.ID})
	if err != nil || catalog.Path != "/workspaces/Local" || catalog.Ref.WorkspaceID != record.ID {
		t.Fatalf("catalog %+v %v", catalog, err)
	}
	page, err := a.ListWorkspaceMounts("/workspaces/Local", 0, 100)
	if err != nil || len(page.Entries) != 1 || page.Entries[0].Path != "/workspaces/Local/Archive" {
		t.Fatalf("listing %+v %v", page, err)
	}
	if _, err := a.RenameWorkspaceSource(record.ID, source.ID, source.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AttachLocalWorkspaceSource(record.ID, t.TempDir(), "Archive"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StatWorkspaceMountExpected(current.Path, ref); err == nil {
		t.Fatal("reused source name redirected retained resource")
	}
	resolved, err := a.ResolveWorkspaceResourceMount(ref)
	if err != nil || resolved.Ref.SourceID != source.ID || resolved.Path == current.Path {
		t.Fatalf("retained resource %+v %v", resolved, err)
	}
}
