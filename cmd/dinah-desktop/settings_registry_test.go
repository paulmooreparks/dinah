package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"studio/internal/workspaces"
)

// fill sets v to a value that is not the zero value, as far as its kind
// allows, so a copy that drops it can be seen. It reports whether it could.
func fill(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(3)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(3)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	case reflect.Slice:
		if v.Type() == reflect.TypeFor[json.RawMessage]() {
			v.Set(reflect.ValueOf(json.RawMessage(`1`)))
			return true
		}
		s := reflect.MakeSlice(v.Type(), 1, 1)
		if !fill(s.Index(0)) {
			return false
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key, elem := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		if !fill(key) || !fill(elem) {
			return false
		}
		m.SetMapIndex(key, elem)
		v.Set(m)
	case reflect.Struct:
		any := false
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() && fill(v.Field(i)) {
				any = true
			}
		}
		return any
	default:
		return false
	}
	return true
}

// clone is how updateSettings keeps a change from reaching the settings
// until it is saved, so a field it dropped would be lost on the next change.
// Every exported field is set in turn, and none may come back different.
func TestSettingsCloneKeepsEveryField(t *testing.T) {
	typ := reflect.TypeFor[Settings]()
	checked := 0
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		var s Settings
		if !fill(reflect.ValueOf(&s).Elem().Field(i)) {
			t.Logf("not checked, a kind the filler cannot set: %s", field.Name)
			continue
		}
		got, err := s.clone()
		if err != nil {
			t.Fatalf("%s: %v", field.Name, err)
		}
		if !reflect.DeepEqual(reflect.ValueOf(got).Field(i).Interface(), reflect.ValueOf(s).Field(i).Interface()) {
			t.Errorf("clone changed %s: %#v became %#v", field.Name, reflect.ValueOf(s).Field(i).Interface(), reflect.ValueOf(got).Field(i).Interface())
		}
		checked++
	}
	if checked < 20 {
		t.Fatalf("only %d fields were checked", checked)
	}
	// The registry is the field that is not in the saved form.
	s := Settings{WorkspaceRecords: []WorkspaceRecord{{ID: "ws_1", Name: "One", Sources: []workspaces.Source{{ID: "src"}}}}}
	got, err := s.clone()
	if err != nil || len(got.WorkspaceRecords) != 1 {
		t.Fatalf("the registry was not cloned: %+v %v", got.WorkspaceRecords, err)
	}
	got.WorkspaceRecords[0].Sources[0].ID = "changed"
	if s.WorkspaceRecords[0].Sources[0].ID != "src" {
		t.Fatal("the clone shares a source with the original")
	}
}

func readConfigFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(configDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// legacyRegistrySettings writes a settings.json in the form earlier
// releases saved, with the registry inside it.
func legacyRegistrySettings(t *testing.T, folders ...string) ([]byte, []WorkspaceRecord) {
	t.Helper()
	var records []WorkspaceRecord
	for _, folder := range folders {
		records = append(records, newWorkspaceRecord(folder))
	}
	data, err := json.MarshalIndent(map[string]any{"workspace": folders[0], "workspaceId": records[0].ID, "workspaces": folders, "workspaceRecords": records}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return data, records
}

// A settings.json that still holds the registry has it moved to its own
// file once, with a copy of the old settings kept for restoring by hand,
// and a second start changes nothing.
func TestRegistryMovesOutOfSettings(t *testing.T) {
	isolateWorkspaceSettings(t)
	one, two := t.TempDir(), t.TempDir()
	original, _ := legacyRegistrySettings(t, one, two)

	a := closeBusWhenDone(t, NewApp())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	state := a.State()
	if len(state.WorkspaceRecords) != 2 {
		t.Fatalf("the registry did not survive the move: %+v", state.WorkspaceRecords)
	}
	moved, err := workspaces.Load(configDir())
	if err != nil || !workspaces.Same(moved, state.WorkspaceRecords) {
		t.Fatalf("workspaces.json: %+v %v", moved, err)
	}
	if strings.Contains(string(readConfigFile(t, "settings.json")), "workspaceRecords") {
		t.Fatal("settings.json still holds the registry")
	}
	if string(readConfigFile(t, "settings.json.before-split")) != string(original) {
		t.Fatal("the copy of the old settings is not the old settings")
	}

	settings, registry, backup := readConfigFile(t, "settings.json"), readConfigFile(t, workspaces.RegistryFile), readConfigFile(t, "settings.json.before-split")
	b := closeBusWhenDone(t, NewApp())
	if !reflect.DeepEqual(b.State().WorkspaceRecords, state.WorkspaceRecords) {
		t.Fatal("a second start read a different registry")
	}
	if string(readConfigFile(t, "settings.json")) != string(settings) || string(readConfigFile(t, workspaces.RegistryFile)) != string(registry) || string(readConfigFile(t, "settings.json.before-split")) != string(backup) {
		t.Fatal("a second start changed a file")
	}
}

// Where both the file and the old copy exist, which is where a start was
// interrupted between the steps, the file is read and the old copy goes.
func TestRegistryFileWinsOverTheOldCopy(t *testing.T) {
	isolateWorkspaceSettings(t)
	one, two := t.TempDir(), t.TempDir()
	_, records := legacyRegistrySettings(t, one, two)
	// The file has the same workspaces, one of them renamed since the old
	// copy was made.
	newer := workspaces.Copy(records)
	newer[0].Name = "Renamed in the file"
	if err := workspaces.Save(configDir(), newer); err != nil {
		t.Fatal(err)
	}
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	if got := a.State().WorkspaceRecords; len(got) != 2 || got[0].Name != "Renamed in the file" {
		t.Fatalf("the old copy was read: %+v", got)
	}
	if strings.Contains(string(readConfigFile(t, "settings.json")), "workspaceRecords") {
		t.Fatal("settings.json still holds the registry")
	}
	// The registry was already in its own file, so this is not the move
	// that the copy is for.
	if _, err := os.Stat(filepath.Join(configDir(), "settings.json.before-split")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a copy was made though the registry had already moved")
	}
}

// With nothing to move, the registry file is still made, empty, so the
// command line can tell a registry with no workspaces from one not moved.
func TestFreshStartMakesAnEmptyRegistryFile(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	records, err := workspaces.Load(configDir())
	if err != nil || len(records) != 0 {
		t.Fatalf("a fresh start left the registry %+v, %v", records, err)
	}
	if _, err := os.Stat(filepath.Join(configDir(), "settings.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a fresh start wrote settings.json")
	}
}

// An unreadable registry stops the start with its own message and leaves
// both files alone.
func TestACorruptRegistryStopsTheStart(t *testing.T) {
	isolateWorkspaceSettings(t)
	if err := os.WriteFile(filepath.Join(configDir(), workspaces.RegistryFile), []byte(`{"workspaces":`), 0o600); err != nil {
		t.Fatal(err)
	}
	a := closeBusWhenDone(t, NewApp())
	if a.startupErr == nil || !strings.Contains(a.startupErr.Error(), "workspace registry") {
		t.Fatalf("start error: %v", a.startupErr)
	}
}

// registrySave is one write of the registry that saveSettings made, with the
// IDs it held and settings.json as it was on disk at that moment.
type registrySave struct {
	ids      []string
	settings string
}

// watchRegistry replaces the registry write with one that records each call
// and then does what fail says, or the real write. It restores the real
// one when the test ends.
func watchRegistry(t *testing.T, fail func(call int, ids []string) error) *[]registrySave {
	t.Helper()
	var saves []registrySave
	real := saveRegistry
	t.Cleanup(func() { saveRegistry = real })
	saveRegistry = func(dir string, records []workspaces.Record) error {
		var ids []string
		for _, r := range records {
			ids = append(ids, r.ID)
		}
		on, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
		saves = append(saves, registrySave{ids, string(on)})
		if fail != nil {
			if err := fail(len(saves), ids); err != nil {
				return err
			}
		}
		return real(dir, records)
	}
	return &saves
}

func holds(ids []string, id string) bool { return slices.Contains(ids, id) }

// A new workspace reaches the registry before the settings refer to it, in
// one registry write. If that write fails, settings.json is untouched; if
// writing settings.json fails, the registry is put back as it was.
func TestAddingAWorkspaceSavesTheRegistryFirst(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	first, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	saves := watchRegistry(t, nil)
	state, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	added := state.WorkspaceID
	if len(*saves) != 1 || !holds((*saves)[0].ids, added) || strings.Contains((*saves)[0].settings, added) {
		t.Fatalf("the registry was not saved once, before the settings refer to the new workspace: %+v", *saves)
	}
	if !strings.Contains(string(readConfigFile(t, "settings.json")), added) {
		t.Fatal("settings.json does not refer to the new workspace")
	}

	// The registry cannot be written: nothing else is.
	settings, registry := readConfigFile(t, "settings.json"), readConfigFile(t, workspaces.RegistryFile)
	fails := true
	watchRegistry(t, func(int, []string) error {
		if fails {
			return errors.New("registry unwritable")
		}
		return nil
	})
	if _, err := a.SetWorkspace(t.TempDir()); err == nil {
		t.Fatal("the add hid a failed registry save")
	}
	if string(readConfigFile(t, "settings.json")) != string(settings) || string(readConfigFile(t, workspaces.RegistryFile)) != string(registry) {
		t.Fatal("a failed registry save changed a file")
	}

	// settings.json cannot be written: the registry goes back.
	fails = false
	if err := os.Mkdir(filepath.Join(configDir(), "settings.json.pending"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetWorkspace(t.TempDir()); err == nil {
		t.Fatal("the add hid a failed settings save")
	}
	if string(readConfigFile(t, workspaces.RegistryFile)) != string(registry) {
		t.Fatal("a failed settings save left the new record in the registry")
	}
	if got := a.State().WorkspaceRecords; len(got) != 2 || got[0].ID != first.WorkspaceID && got[1].ID != first.WorkspaceID {
		t.Fatalf("a failed add changed the state: %+v", got)
	}
}

// Removing a workspace takes it out of the settings before the registry. With
// the registry unwritable the removal still reaches the settings, the record
// stays in the file with nothing referring to it, the window starts cleanly
// on that, and the next save that can write the registry drops the record.
func TestRemovingAWorkspaceDropsTheSettingsFirst(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	keep, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	goneID := state.WorkspaceID

	saves := watchRegistry(t, func(int, []string) error { return errors.New("registry unwritable") })
	if _, err := a.RemoveWorkspace(goneID); err != nil {
		t.Fatalf("the removal failed on the registry's last step: %v", err)
	}
	if len(*saves) != 1 || holds((*saves)[0].ids, goneID) || strings.Contains((*saves)[0].settings, goneID) {
		t.Fatalf("the registry was not written once, after settings.json stopped referring to the workspace: %+v", *saves)
	}
	if strings.Contains(string(readConfigFile(t, "settings.json")), goneID) {
		t.Fatal("settings.json still refers to the removed workspace")
	}
	left, err := workspaces.Load(configDir())
	if err != nil || len(left) != 2 {
		t.Fatalf("the registry file should still hold both records, has %+v %v", left, err)
	}

	// A start on that state: the unreferenced record is accepted. It is a
	// registered workspace again, since the registry is what lists them.
	b := closeBusWhenDone(t, NewApp())
	if b.startupErr != nil {
		t.Fatalf("an unreferenced record stopped the start: %v", b.startupErr)
	}
	back := false
	for _, r := range b.State().WorkspaceRecords {
		back = back || r.ID == goneID
	}
	if !back {
		t.Fatal("the leftover record was not loaded")
	}

	// In the run that failed, the next save drops the record.
	*saves = nil
	saveRegistry = workspaces.Save
	if err := a.updateSettings(func(s *Settings) error { s.NewWindowMode = "maximized"; return nil }); err != nil {
		t.Fatal(err)
	}
	left, err = workspaces.Load(configDir())
	if err != nil || len(left) != 1 || left[0].ID != keep.WorkspaceID {
		t.Fatalf("the next save did not drop the leftover record: %+v %v", left, err)
	}
}
