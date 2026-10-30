package main

import (
	"os"
	"path/filepath"
	"testing"

	"studio/internal/workspaces"
)

func TestWorkspaceEmptyCreationRestartAndSources(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	state, err := a.CreateWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	record := state.WorkspaceRecords[0]
	if record.Name != "Workspace" || len(record.Sources) != 0 || record.PrimarySourceID != "" || state.WorkspaceID != record.ID {
		t.Fatalf("new workspace is not an empty named collection: %+v", record)
	}
	expected := filepath.Join(configDir(), "workspaces", record.ID, "files")
	if state.Workspace != expected || record.ExecutionPath != expected {
		t.Fatal("execution storage is not separate application data")
	}
	if _, err := os.Stat(expected); err != nil {
		t.Fatal(err)
	}
	first, second := t.TempDir(), t.TempDir()
	if _, err := a.AttachLocalWorkspaceSource(record.ID, first, "First Folder"); err != nil {
		t.Fatal(err)
	}
	attached, err := a.AttachLocalWorkspaceSource(record.ID, second, "Second Folder")
	if err != nil {
		t.Fatal(err)
	}
	if len(attached.Sources) != 2 || attached.Name != "Workspace" || a.State().Workspace != expected {
		t.Fatal("attaching sources changed workspace identity or storage")
	}
	for _, source := range attached.Sources {
		if _, err := a.DetachWorkspaceSource(record.ID, source.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(source.Path); err != nil {
			t.Fatal("detaching deleted source", err)
		}
	}
	if _, err := a.RenameWorkspace(record.ID, "Meeting Notes"); err != nil {
		t.Fatal(err)
	}
	other, err := a.CreateWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if other.WorkspaceID == record.ID {
		t.Fatal("new workspace reused identity")
	}
	restored := closeBusWhenDone(t, NewApp())
	if restored.startupErr != nil {
		t.Fatal(restored.startupErr)
	}
	selected, err := restored.SelectWorkspace(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	index := workspaces.Index(selected.WorkspaceRecords, record.ID)
	if index < 0 || selected.WorkspaceRecords[index].Name != "Meeting Notes" || len(selected.WorkspaceRecords[index].Sources) != 0 || selected.Workspace != expected {
		t.Fatal("empty workspace did not survive restart or selection")
	}
	if len(selected.WorkspaceRecords) != 2 {
		t.Fatal("execution storage was imported as another workspace")
	}
}

func TestWorkspaceEmptyCreationSaveFailureKeepsSelection(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	before, err := a.CreateWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(configDir(), "settings.json")
	if err := os.Remove(settingsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(settingsPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateWorkspace(); err == nil {
		t.Fatal("save failure accepted")
	}
	after := a.State()
	if after.WorkspaceID != before.WorkspaceID || len(after.WorkspaceRecords) != len(before.WorkspaceRecords) {
		t.Fatal("failed save changed registry or selection")
	}
}
