package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/workspaces"
)

// workspace export writes the export the window's Import Workspace… reads,
// found by the workspace's name, keeps a source of a kind this build does
// not know as it is, and changes nothing.
func TestWorkspaceExportWritesTheStructure(t *testing.T) {
	home := t.TempDir()
	ws := filepath.Join(home, "OneDrive", "Orchard")
	if err := os.MkdirAll(filepath.Join(ws, "Concepts"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(home, product.HomeFolder)
	os.MkdirAll(state, 0o700)
	t.Setenv(product.HomeVar(), state)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	// Before the window has moved the registry to its own file, the command
	// line says so; it does not list the workspaces as none.
	var notMoved bytes.Buffer
	if err := workspaceCommand([]string{"list"}, &notMoved); err == nil || !strings.Contains(err.Error(), "has not been moved") || notMoved.Len() != 0 {
		t.Fatalf("before the move: %v %q", err, notMoved.String())
	}
	settings, _ := json.Marshal(map[string]any{"workspaces": []map[string]any{{"id": "ws_test", "name": "Orchard", "primarySourceId": "src", "executionPath": ws,
		"sources": []map[string]any{{"id": "src", "kind": "local-folder", "name": "Orchard", "path": ws}, {"id": "j", "kind": "tracker-project", "name": "ORCH", "baseUrl": "https://tracker.example.com", "projectKey": "ORCH"}}}}})
	os.WriteFile(filepath.Join(state, workspaces.RegistryFile), settings, 0o600)

	var out bytes.Buffer
	if err := workspaceCommand([]string{"list"}, &out); err != nil || !strings.Contains(out.String(), `"ws_test"`) {
		t.Fatalf("list: %v %s", err, out.String())
	}
	file := filepath.Join(ws, "Orchard"+workspaces.ExportExtension)
	out.Reset()
	if err := workspaceCommand([]string{"export", "orchard", "--out", file}, &out); err != nil {
		t.Fatalf("export: %v %s", err, out.String())
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	export, err := workspaces.ReadExport(data)
	if err != nil || export.Name != "Orchard" || len(export.Sources) != 2 || !export.Sources[0].Main || export.Sources[0].Local.FromHome != "OneDrive/Orchard" || export.Sources[0].Local.Folders[0] != "Concepts" {
		t.Fatalf("export: %+v %v", export, err)
	}
	if string(export.Sources[1].Extra["projectKey"]) != `"ORCH"` {
		t.Fatalf("the source of a kind this build does not know lost its fields: %s", data)
	}
	if after, _ := os.ReadFile(filepath.Join(state, workspaces.RegistryFile)); string(after) != string(settings) {
		t.Fatal("the export changed the registry")
	}
	out.Reset()
	if err := workspaceCommand([]string{"export", "--out", file + ".2", "ws_test"}, &out); err != nil {
		t.Fatalf("export by ID with --out first: %v", err)
	}
	if err := workspaceCommand([]string{"export", "Nothing"}, &out); err == nil || !strings.Contains(err.Error(), "workspace list") {
		t.Fatalf("unknown workspace: %v", err)
	}
	if err := workspaceCommand([]string{"import", file}, &out); err == nil {
		t.Fatal("the command line imported a workspace")
	}
}

// A registry that exists and holds no workspaces lists as empty, which is
// what tells it from one the window has not moved yet. Nothing in the
// window's settings is read.
func TestWorkspaceListOfAnEmptyRegistry(t *testing.T) {
	state := t.TempDir()
	t.Setenv(product.HomeVar(), state)
	os.WriteFile(filepath.Join(state, "settings.json"), []byte(`{"workspaceRecords":[{"id":"ws_old","name":"Old","executionPath":"C:/Old","sources":[]}]}`), 0o600)
	var out bytes.Buffer
	if err := workspaceCommand([]string{"list"}, &out); err == nil || !strings.Contains(err.Error(), "has not been moved") {
		t.Fatalf("with only the old settings: %v %q", err, out.String())
	}
	if err := workspaces.Save(state, nil); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := workspaceCommand([]string{"list"}, &out); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("empty registry: %v %q", err, out.String())
	}
}
