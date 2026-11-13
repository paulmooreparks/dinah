package drafts

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A draft is saved although a reader has its file open at that moment. On
// Windows the rename over the file is refused while the reader holds it, and
// the save waits the reader out. A save was lost to that refusal once
// (Paul's log, 12 November 2026).
func TestADraftIsSavedWhileItsFileIsBeingRead(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	text := "one\n"
	d, err := store.Create(Origin{Kind: OriginNew, WorkspaceID: "w"}, Baseline{}, "Untitled", &text)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(filepath.Join(dir, d.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	released := time.AfterFunc(300*time.Millisecond, func() { reader.Close() })
	defer released.Stop()
	defer reader.Close()
	saved, err := store.Update(d.ID, d.Revision, "two\n", nil)
	if err != nil {
		t.Fatalf("a save while the file was read: %v", err)
	}
	if got, err := store.Get(d.ID); err != nil || got.Text != "two\n" || got.Revision != saved.Revision {
		t.Fatalf("the saved draft: %+v %v", got, err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) != 0 {
		t.Fatalf("temporary files were left: %v", left)
	}
}

// A file that stays held longer than the wait is reported, and the draft is
// as it was.
func TestASaveRefusedForLongerThanTheWaitIsReported(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	store.lockWait = 100 * time.Millisecond
	text := "one\n"
	d, err := store.Create(Origin{Kind: OriginNew, WorkspaceID: "w"}, Baseline{}, "Untitled", &text)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(filepath.Join(dir, d.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Update(d.ID, d.Revision, "two\n", nil)
	reader.Close()
	if err == nil {
		t.Skip("this system replaces a file that is open, so there is no refusal to report")
	}
	if got, gerr := store.Get(d.ID); gerr != nil || got.Text != "one\n" {
		t.Fatalf("the draft after a refused save: %+v %v", got, gerr)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) != 0 {
		t.Fatalf("temporary files were left: %v", left)
	}
}
