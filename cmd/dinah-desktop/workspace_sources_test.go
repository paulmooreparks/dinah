package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"studio/internal/workspaces"
)

func TestWorkspaceSourcesMigrationAndPrimaryOwnership(t *testing.T) {
	isolateWorkspaceSettings(t)
	first, second, attached := t.TempDir(), t.TempDir(), t.TempDir()
	legacy := Settings{Workspace: first, Workspaces: []string{first, second}, WorkspaceRecords: []WorkspaceRecord{
		{ID: "first", Name: "First Work", Sources: []WorkspaceSource{{ID: "first-root", Kind: "local-folder", Path: first}}},
		{ID: "second", Name: "Second Work", Sources: []WorkspaceSource{{ID: "second-root", Kind: "local-folder", Path: second}}},
	}}
	if err := saveSettings(legacy); err != nil {
		t.Fatal(err)
	}
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	if record := a.State().WorkspaceRecords[0]; record.PrimarySourceID != "first-root" || record.Sources[0].Name != "First Work" {
		t.Fatalf("legacy primary was not migrated: %+v", record)
	}
	if _, err := a.SelectWorkspace("second"); err != nil {
		t.Fatal(err)
	}
	record, err := a.AttachLocalWorkspaceSource("first", attached, "References")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Sources) != 2 || a.State().WorkspaceID != "second" || a.State().Workspace != second {
		t.Fatal("inactive attachment changed the selected workspace")
	}
	duplicate, err := a.AttachLocalWorkspaceSource("first", attached+string(filepath.Separator)+".", "Ignored Duplicate Label")
	if err != nil || !reflect.DeepEqual(record, duplicate) {
		t.Fatalf("same attachment changed identity: %+v %v", duplicate, err)
	}
	shared, err := a.AttachLocalWorkspaceSource("second", attached, "Shared References")
	if err != nil || shared.Sources[1].ID == record.Sources[1].ID {
		t.Fatalf("shared attachment did not get workspace-local identity: %+v %v", shared, err)
	}
	if workspaces.IndexAt(a.State().WorkspaceRecords, attached) != -1 {
		t.Fatal("attached folder was treated as an execution workspace")
	}
	third, err := a.SetWorkspace(attached)
	if err != nil || third.WorkspaceID == "first" || third.WorkspaceID == "second" || len(third.WorkspaceRecords) != 3 {
		t.Fatalf("opening attachment reused its owner's execution identity: %+v %v", third, err)
	}
	before := a.State().WorkspaceRecords
	restarted := closeBusWhenDone(t, NewApp())
	if restarted.startupErr != nil || !reflect.DeepEqual(before, restarted.State().WorkspaceRecords) {
		t.Fatalf("multi-source registry did not survive restart: %v", restarted.startupErr)
	}
	// Moving the primary within the list must not change execution ownership.
	item := before[0]
	item.Sources[0], item.Sources[1] = item.Sources[1], item.Sources[0]
	if source, ok := workspaces.PrimarySource(item); !ok || source.Path != first || workspaces.IndexAt([]WorkspaceRecord{item}, attached) != -1 {
		t.Fatal("source ordering changed primary ownership")
	}
}

func TestWorkspaceSourceDetachDoesNotDeleteAndSaveFailureIsIsolated(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	state, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	content := filepath.Join(folder, "notes.txt")
	if err := os.WriteFile(content, []byte("preserve this file"), 0o600); err != nil {
		t.Fatal(err)
	}
	record, err := a.AttachLocalWorkspaceSource(state.WorkspaceID, folder, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.DetachWorkspaceSource(record.ID, record.PrimarySourceID); err != nil {
		t.Fatal(err)
	}
	if a.State().Workspace != state.Workspace {
		t.Fatal("detaching a source redirected execution storage")
	}
	if _, err := a.DetachWorkspaceSource(record.ID, record.Sources[1].ID); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(content)
	if err != nil || string(data) != "preserve this file" {
		t.Fatalf("detach changed source content: %v", err)
	}
	record, err = a.AttachLocalWorkspaceSource(state.WorkspaceID, folder, "")
	if err != nil {
		t.Fatal(err)
	}
	before := a.State()
	if err := os.Mkdir(filepath.Join(configDir(), "settings.json.pending"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DetachWorkspaceSource(record.ID, record.Sources[0].ID); err == nil {
		t.Fatal("detach ignored save failure")
	}
	if _, err := a.AttachLocalWorkspaceSource(record.ID, t.TempDir(), ""); err == nil {
		t.Fatal("attach ignored save failure")
	}
	if !reflect.DeepEqual(before, a.State()) {
		t.Fatal("failed mutation changed in-memory source registry")
	}
}

// The identity action lists a workspace's local sources. A registry with a
// source of a kind this build does not know, which an import kept, loads
// with the source as it is, and one whose kind cannot be a kind, or whose
// unknown source has a path, fails to load rather than losing it.
func TestWorkspaceIdentityListsLocalSources(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	state, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.AttachLocalWorkspaceSource(state.WorkspaceID, t.TempDir(), "References"); err != nil {
		t.Fatal(err)
	}
	kept := WorkspaceSource{ID: "s", Kind: "unknown-kind", Name: "S", Extra: map[string]json.RawMessage{"pageId": json.RawMessage(`"1"`)}}
	if migrated, _, err := migrateWorkspaceSettings(Settings{WorkspaceRecords: []WorkspaceRecord{{ID: "w", Name: "W", ExecutionPath: t.TempDir(), Sources: []WorkspaceSource{kept}}}}); err != nil || string(migrated.WorkspaceRecords[0].Sources[0].Extra["pageId"]) != `"1"` {
		t.Fatalf("a source of a kind this build does not know was not kept: %v", err)
	}
	for _, bad := range []WorkspaceSource{{ID: "s", Kind: "Unknown Kind", Name: "S"}, {ID: "s", Kind: "unknown-kind", Name: "S", Path: t.TempDir()}} {
		if _, _, err := migrateWorkspaceSettings(Settings{WorkspaceRecords: []WorkspaceRecord{{ID: "w", Name: "W", ExecutionPath: t.TempDir(), Sources: []WorkspaceSource{bad}}}}); err == nil {
			t.Fatalf("an invalid source was accepted: %+v", bad)
		}
	}
	bundle, err := a.workspaceAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	result, err := bundle.Handlers[workspaceIdentityAction](builtinTestContext(state.Workspace), `{}`)
	var identity struct {
		Sources []WorkspaceSource `json:"sources"`
	}
	if err != nil || json.Unmarshal([]byte(result.OutputJSON), &identity) != nil || len(identity.Sources) != 2 || identity.Sources[0].Kind != "local-folder" || identity.Sources[1].Kind != "local-folder" {
		t.Fatalf("describe@2 strict local-source schema changed: %s %v", result.OutputJSON, err)
	}
}
