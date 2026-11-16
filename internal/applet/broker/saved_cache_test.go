package broker

import (
	"os"
	"path/filepath"
	"testing"

	"studio/internal/layout"
)

// Saved gives what it kept while the record files are unchanged, and reads
// again as soon as one of them changes.
func TestSavedIsReadAgainOnlyWhenARecordFileChanges(t *testing.T) {
	state, workspace := t.TempDir(), t.TempDir()
	b := &Broker{State: state}
	dir := layout.Layout{State: state}.RunDir(b.StoreRoot(workspace))
	first, err := b.Saved(workspace)
	if err != nil || len(first) != 0 {
		t.Fatalf("an empty store gave %v, %v", first, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := savedStamp(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := savedStamp(dir)
	if err != nil || after == before {
		t.Fatalf("a new file did not change the stamp: %v", err)
	}
	again, err := savedStamp(dir)
	if err != nil || again != after {
		t.Fatal("an unchanged folder changed its stamp")
	}
}
