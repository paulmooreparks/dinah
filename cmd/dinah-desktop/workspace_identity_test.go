package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"studio/internal/applet"
	"studio/internal/workspaces"
)

func isolateWorkspaceSettings(t *testing.T) {
	t.Helper()
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
}

func TestWorkspaceIdentityMigrationPreservesPathsAndMissingRoots(t *testing.T) {
	isolateWorkspaceSettings(t)
	first, second := t.TempDir(), t.TempDir()
	missing := filepath.Join(t.TempDir(), "offline-root")
	legacy := Settings{Workspace: first, Workspaces: []string{first, first + string(filepath.Separator) + ".", second}, Recent: []string{missing, second, first}}
	if err := saveSettings(legacy); err != nil {
		t.Fatal(err)
	}
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	state := a.State()
	if len(state.WorkspaceRecords) != 3 || state.WorkspaceID == "" || state.Workspace != first {
		t.Fatalf("migration identity: %+v", state.WorkspaceRecords)
	}
	if !reflect.DeepEqual(state.Workspaces, legacy.Workspaces) || !reflect.DeepEqual(state.Recent, legacy.Recent) {
		t.Fatal("migration changed legacy path keys")
	}
	missingIndex := workspaces.IndexAt(state.WorkspaceRecords, missing)
	if missingIndex < 0 {
		t.Fatal("offline root discarded")
	}
	restarted := closeBusWhenDone(t, NewApp())
	if restarted.startupErr != nil || !reflect.DeepEqual(state.WorkspaceRecords, restarted.State().WorkspaceRecords) || restarted.State().WorkspaceID != state.WorkspaceID {
		t.Fatal("restart replaced workspace or source identities")
	}
	before := restarted.State()
	if _, err := restarted.SelectWorkspace(state.WorkspaceRecords[missingIndex].ID); err == nil {
		t.Fatal("selected unavailable root")
	}
	if !reflect.DeepEqual(before, restarted.State()) {
		t.Fatal("failed selection changed state or discarded missing identity")
	}
}

func TestWorkspaceIdentityRenameAndSelectionAreIndependent(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	first, second := t.TempDir(), t.TempDir()
	one, err := a.SetWorkspace(first)
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.SetWorkspace(second)
	if err != nil {
		t.Fatal(err)
	}
	before := copyWorkspaceSettings(a.settings)
	renamed, err := a.RenameWorkspace(one.WorkspaceID, "Writing")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.ID != one.WorkspaceID || !reflect.DeepEqual(renamed.Sources[0], one.WorkspaceRecords[0].Sources[0]) || a.State().WorkspaceID != two.WorkspaceID || a.State().Workspace != second {
		t.Fatal("rename changed identity, source or selection")
	}
	if !reflect.DeepEqual(before.Workspaces, a.settings.Workspaces) || !reflect.DeepEqual(before.Recent, a.settings.Recent) || before.WorkspaceRecords[0].Name == "Writing" {
		t.Fatal("rename mutated projections or prior settings")
	}
	renamed.Sources[0].Path = "changed by caller"
	if a.State().WorkspaceRecords[0].Sources[0].Path != first {
		t.Fatal("rename result aliases stored sources")
	}
	selected, err := a.SelectWorkspace(one.WorkspaceID)
	if err != nil || selected.Workspace != first || selected.WorkspaceID != one.WorkspaceID || selected.WorkspaceRecords[0].Name != "Writing" {
		t.Fatalf("selection: %+v %v", selected, err)
	}
	resolved, err := a.SetWorkspace(first + string(filepath.Separator) + ".")
	if err != nil || resolved.WorkspaceID != one.WorkspaceID || len(resolved.WorkspaceRecords) != 2 {
		t.Fatal("path resolution created a duplicate identity")
	}
	if closeBusWhenDone(t, NewApp()).State().WorkspaceRecords[0].Name != "Writing" {
		t.Fatal("display name did not survive restart")
	}
}

func TestWorkspaceIdentitySaveFailuresDoNotChangeState(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	one, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before := a.State()
	saved, err := os.ReadFile(filepath.Join(configDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(configDir(), "settings.json.pending"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenameWorkspace(one.WorkspaceID, "Must Not Persist"); err == nil {
		t.Fatal("rename hid failed save")
	}
	if _, err := a.SelectWorkspace(one.WorkspaceID); err == nil {
		t.Fatal("selection hid failed save")
	}
	if _, err := a.SetWorkspace(t.TempDir()); err == nil {
		t.Fatal("new identity hid failed save")
	}
	if _, err := a.ForgetWorkspace(two.Workspace); err == nil {
		t.Fatal("recent change hid failed save")
	}
	if !reflect.DeepEqual(before, a.State()) {
		t.Fatal("failed save changed in-memory state")
	}
	after, _ := os.ReadFile(filepath.Join(configDir(), "settings.json"))
	if string(after) != string(saved) {
		t.Fatal("failed save changed legacy settings")
	}
	if closeBusWhenDone(t, NewApp()).startupErr == nil {
		t.Fatal("startup hid unreadable migration storage")
	}
}

func TestWorkspaceIdentityInvalidRegistryPreservesLegacySettings(t *testing.T) {
	isolateWorkspaceSettings(t)
	path := t.TempDir()
	legacy := Settings{Workspace: path, WorkspaceID: "missing-identity", Recent: []string{path}}
	if err := saveSettings(legacy); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(configDir(), "settings.json"))
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr == nil || a.settings.Workspace != path || len(a.settings.WorkspaceRecords) != 0 {
		t.Fatal("invalid registry silently replaced identity or legacy settings")
	}
	if _, err := a.SetWorkspace(t.TempDir()); err == nil {
		t.Fatal("mutation ignored the startup migration error")
	}
	after, _ := os.ReadFile(filepath.Join(configDir(), "settings.json"))
	if string(before) != string(after) {
		t.Fatal("failed migration overwrote settings")
	}
}

func TestWorkspaceIdentityAppletVersionsAndScope(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	path := t.TempDir()
	state, err := a.SetWorkspace(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenameWorkspace(state.WorkspaceID, "Parser Work"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetWorkspace(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	bundle, err := a.workspaceAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(path)
	legacy, err := r.Execute(ctx, workspaceDescribeAction, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	_ = json.Unmarshal([]byte(legacy.OutputJSON), &old)
	if len(old) != 1 || old["name"] != filepath.Base(path) {
		t.Fatalf("describe@1 contract changed: %s", legacy.OutputJSON)
	}
	identity, err := r.Execute(ctx, workspaceIdentityAction, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var current struct {
		ID      string              `json:"id"`
		Name    string              `json:"name"`
		Sources []map[string]string `json:"sources"`
	}
	if err := json.Unmarshal([]byte(identity.OutputJSON), &current); err != nil {
		t.Fatal(err)
	}
	if current.ID != state.WorkspaceID || current.Name != "Parser Work" || len(current.Sources) != 1 || len(current.Sources[0]) != 3 || current.Sources[0]["kind"] != "local-folder" {
		t.Fatalf("incorrect invocation identity: %s", identity.OutputJSON)
	}
	if strings.Contains(identity.OutputJSON, `"path"`) || strings.Contains(identity.OutputJSON, filepath.VolumeName(path)) && filepath.VolumeName(path) != "" {
		t.Fatal("describe@2 exposed an absolute host path")
	}
	if _, err := r.Execute(ctx, workspaceIdentityAction, `{"workspace":"override"}`); err == nil {
		t.Fatal("caller changed workspace scope")
	}
	if _, err := r.Execute(context.Background(), workspaceIdentityAction, `{}`); err == nil {
		t.Fatal("unbound invocation accepted")
	}
	if _, err := r.Execute(builtinTestContext(t.TempDir()), workspaceIdentityAction, `{}`); err == nil {
		t.Fatal("pure action created an unregistered workspace")
	}
}

func TestWorkspaceIdentityPathCaseAndRootName(t *testing.T) {
	root := t.TempDir()
	upper, lower := filepath.Join(root, "Data"), filepath.Join(root, "data")
	settings, _, err := migrateWorkspaceSettings(Settings{Workspaces: []string{upper, lower}})
	if err != nil {
		t.Fatal(err)
	}
	want := 2
	if runtime.GOOS == "windows" {
		want = 1
	}
	if len(settings.WorkspaceRecords) != want {
		t.Fatalf("case equality on %s merged %d roots; want %d", runtime.GOOS, len(settings.WorkspaceRecords), want)
	}
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	record := newWorkspaceRecord(volumeRoot)
	if !validWorkspaceName(record.Name) || filepath.IsAbs(record.Name) {
		t.Fatalf("invalid root display name %q", record.Name)
	}
}

func TestWorkspaceIdentityGeneratedNamesSurviveRestart(t *testing.T) {
	for _, migrate := range []bool{false, true} {
		t.Run(map[bool]string{false: "select", true: "migrate"}[migrate], func(t *testing.T) {
			isolateWorkspaceSettings(t)
			path := filepath.Join(t.TempDir(), " Leading")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if migrate {
				if err := saveSettings(Settings{Workspace: path}); err != nil {
					t.Fatal(err)
				}
			}
			a := closeBusWhenDone(t, NewApp())
			if !migrate {
				if _, err := a.SetWorkspace(path); err != nil {
					t.Fatal(err)
				}
			}
			if a.startupErr != nil {
				t.Fatal(a.startupErr)
			}
			record := a.State().WorkspaceRecords[0]
			if record.Name != "Leading" || record.Sources[0].Path != path {
				t.Fatalf("generated name or exact path changed: %+v", record)
			}
			restarted := closeBusWhenDone(t, NewApp())
			if restarted.startupErr != nil || !reflect.DeepEqual(restarted.State().WorkspaceRecords[0], record) {
				t.Fatalf("generated label prevented restart: %v", restarted.startupErr)
			}
		})
	}
	for _, basename := range []string{strings.Repeat("界", 250), " \t\n ", "\x01" + strings.Repeat("a", 199) + " " + strings.Repeat("b", 30) + " "} {
		t.Run("bounded label", func(t *testing.T) {
			isolateWorkspaceSettings(t)
			path := filepath.Join(t.TempDir(), basename)
			if err := saveSettings(Settings{Workspace: path}); err != nil {
				t.Fatal(err)
			}
			a := closeBusWhenDone(t, NewApp())
			if a.startupErr != nil {
				t.Fatal(a.startupErr)
			}
			record := a.State().WorkspaceRecords[0]
			if !validWorkspaceName(record.Name) || record.Sources[0].Path != path {
				t.Fatalf("invalid generated name or changed source: %+v", record)
			}
			restarted := closeBusWhenDone(t, NewApp())
			if restarted.startupErr != nil || !reflect.DeepEqual(restarted.State().WorkspaceRecords[0], record) {
				t.Fatalf("bounded label prevented restart: %v", restarted.startupErr)
			}
		})
	}
}
