package workspaces

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testRecord(id, name string) Record {
	return Record{ID: id, Name: name, PrimarySourceID: "src_" + id, ExecutionPath: "C:/" + name, Sources: []Source{{ID: "src_" + id, Kind: "local-folder", Name: name, Path: "C:/" + name}}}
}

// Save then Load returns what was saved, keeps the fields of a source of a
// kind this build does not know, and tells an empty registry from none.
func TestSaveAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	if err := Save(home, nil); err != nil {
		t.Fatal(err)
	}
	if records, err := Load(home); err != nil || records == nil || len(records) != 0 {
		t.Fatalf("an empty registry reads as %#v, %v", records, err)
	}
	odd := testRecord("ws_2", "Odd")
	if err := json.Unmarshal([]byte(`{"id":"j","kind":"tracker-project","name":"ORCH","projectKey":"ORCH"}`), &odd.Sources[0]); err != nil {
		t.Fatal(err)
	}
	want := []Record{testRecord("ws_1", "One"), odd}
	if err := Save(home, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(home)
	if err != nil || !Same(got, want) || string(got[1].Sources[0].Extra["projectKey"]) != `"ORCH"` {
		t.Fatalf("round trip: %+v %v", got, err)
	}
}

// A save the window left pending is finished by the reader, which is what
// the statefile package documents for a file two programs touch.
func TestLoadFinishesAPendingSave(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, RegistryFile)
	previous, _ := encodeRegistry([]Record{testRecord("ws_1", "One")})
	next, _ := encodeRegistry([]Record{testRecord("ws_1", "One"), testRecord("ws_2", "Two")})
	if err := os.WriteFile(path, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	pending, _ := json.Marshal(struct {
		Version  int    `json:"version"`
		Previous []byte `json:"previous"`
		Next     []byte `json:"next"`
	}{1, previous, next})
	if err := os.WriteFile(path+".pending", pending, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(home)
	if err != nil || len(got) != 2 {
		t.Fatalf("a pending save was not finished: %+v %v", got, err)
	}
	if _, err := os.Stat(path + ".pending"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the pending record is still there")
	}
}

// Load changes nothing when no save is pending.
func TestLoadChangesNoBytes(t *testing.T) {
	home := t.TempDir()
	if err := Save(home, []Record{testRecord("ws_1", "One")}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(home, RegistryFile))
	if _, err := Load(home); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(filepath.Join(home, RegistryFile)); string(after) != string(before) {
		t.Fatal("a read changed the registry")
	}
}

func TestLoadReportsACorruptRegistry(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, RegistryFile), []byte(`{"workspaces":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(home); err == nil || errors.Is(err, ErrNoRegistry) {
		t.Fatalf("a corrupt registry gave %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, RegistryFile), []byte(`{"workspaces":"no"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(home); err == nil {
		t.Fatal("a registry of the wrong shape was read")
	}
}

// Grown keeps every record of next, in order, and appends the ones of old
// that next dropped.
func TestGrown(t *testing.T) {
	one, two, three := testRecord("ws_1", "One"), testRecord("ws_2", "Two"), testRecord("ws_3", "Three")
	renamed := two
	renamed.Name = "Renamed"
	got := Grown([]Record{one, two}, []Record{renamed, three})
	if len(got) != 3 || got[0].Name != "Renamed" || got[1].ID != "ws_3" || got[2].ID != "ws_1" {
		t.Fatalf("grown: %+v", got)
	}
	if !Same(Grown([]Record{one}, []Record{one, two}), []Record{one, two}) {
		t.Fatal("adding alone should grow to exactly next")
	}
	if !Same(Grown([]Record{one, two}, []Record{one}), []Record{one, two}) {
		t.Fatal("removing alone should grow to exactly old")
	}
	// The result shares nothing with its inputs.
	got[0].Sources[0].Name = "Changed"
	if renamed.Sources[0].Name == "Changed" {
		t.Fatal("grown shares a source with next")
	}
}
