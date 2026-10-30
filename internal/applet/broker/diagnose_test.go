package broker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/layout"
)

// TestDiagnoseState reads a state folder's applet records, without changing
// anything, and reports each one Attention would stop on. It runs only when
// DINAH_APPLET_STATE names the folder.
func TestDiagnoseState(t *testing.T) {
	state := os.Getenv("DINAH_APPLET_STATE")
	if state == "" {
		t.Skip("set DINAH_APPLET_STATE to a state folder")
	}
	b := &Broker{State: state}
	entries, err := os.ReadDir(filepath.Join(state, "applets"))
	if err != nil {
		t.Fatal(err)
	}
	results, dismissed := 0, 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(state, "applets", entry.Name())
		ids, err := b.ids(root)
		if err != nil {
			t.Errorf("%s: listing: %v", entry.Name(), err)
			continue
		}
		for _, id := range ids {
			data, err := layout.Layout{State: state}.Runs(root).Read(id)
			if err != nil {
				t.Errorf("%s/%s: read: %v", entry.Name()[:12], id, err)
				continue
			}
			var v Invocation
			if err := json.Unmarshal(data, &v); err != nil {
				t.Errorf("%s/%s: decode: %v", entry.Name()[:12], id, err)
				continue
			}
			if b.StoreRoot(v.Workspace) != root {
				t.Errorf("%s/%s: workspace record mismatch: %q", entry.Name()[:12], id, v.Workspace)
			}
			if !TerminalStatus(v.Status) {
				t.Logf("%s/%s: %s %s, owner %s %s", entry.Name()[:12], id, v.Status, v.Action, v.OwnerKind, v.OwnerID)
			}
			if TerminalStatus(v.Status) {
				if v.ResultDismissed {
					dismissed++
				} else {
					results++
				}
			}
		}
	}
	t.Logf("results shown: %d, dismissed: %d", results, dismissed)
}

// One damaged record is left out of Attention, and the rest are listed.
func TestAttentionSkipsADamagedRecord(t *testing.T) {
	state := t.TempDir()
	b := &Broker{State: state}
	good := Invocation{ID: "a" + strings.Repeat("0", 25), Workspace: filepath.Join(state, "ws"), OwnerKind: "chat", OwnerID: "c1", Action: "x/y@1", Status: "done", Version: 1}
	root := b.StoreRoot(good.Workspace)
	data, _ := json.Marshal(good)
	if err := (layout.Layout{State: state}).Runs(root).Write(good.ID, data); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.ID = "b" + strings.Repeat("0", 25)
	bad.Workspace = filepath.Join(state, "elsewhere")
	data, _ = json.Marshal(bad)
	if err := (layout.Layout{State: state}).Runs(root).Write(bad.ID, data); err != nil {
		t.Fatal(err)
	}
	list, err := b.Attention(func(Summary) bool { return true })
	if err != nil || len(list) != 1 || list[0].ID != good.ID {
		t.Fatalf("%+v %v", list, err)
	}
}
