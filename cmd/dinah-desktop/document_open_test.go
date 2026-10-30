package main

import (
	"os"
	"path/filepath"
	"testing"
)

// From Your Computer opens a file in a workspace's folder as that file, and
// any other text file as a new document; To Your Computer writes a copy and
// replaces a file only when told to.
func TestOpenAndSaveOnTheComputer(t *testing.T) {
	isolateWorkspaceSettings(t)
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# Plan\n"), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(dir)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}

	inside, err := a.OpenComputerFile(record.ID, filepath.Join(dir, "plan.md"))
	if err != nil || inside.Origin.Kind != "local-folder" || inside.Origin.Path != "plan.md" {
		t.Fatalf("inside %+v %v", inside.Origin, err)
	}
	elsewhere := t.TempDir()
	os.WriteFile(filepath.Join(elsewhere, "notes.txt"), []byte("Loose notes.\n"), 0o644)
	loose, err := a.OpenComputerFile(record.ID, filepath.Join(elsewhere, "notes.txt"))
	if err != nil || loose.Origin.Kind != "new" || loose.Origin.WorkspaceID != record.ID || loose.Title != "notes.txt" || loose.Text != "Loose notes.\n" {
		t.Fatalf("loose %+v %v", loose, err)
	}
	os.WriteFile(filepath.Join(elsewhere, "image.bin"), []byte{0x89, 0, 0xff}, 0o644)
	if _, err := a.OpenComputerFile(record.ID, filepath.Join(elsewhere, "image.bin")); err == nil {
		t.Fatal("a binary file was opened")
	}

	copyPath := filepath.Join(elsewhere, "copy.md")
	if err := a.SaveDraftToComputer(inside.ID, copyPath, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(copyPath); string(data) != "# Plan\n" {
		t.Fatalf("copy %q", data)
	}
	if err := a.SaveDraftToComputer(loose.ID, copyPath, false); err != ErrComputerFileExists {
		t.Fatalf("an existing file was replaced unasked: %v", err)
	}
	if err := a.SaveDraftToComputer(loose.ID, copyPath, true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(copyPath); string(data) != "Loose notes.\n" {
		t.Fatalf("replaced copy %q", data)
	}
	if again, _ := a.GetDraft(inside.ID); again.Origin.Path != "plan.md" {
		t.Fatalf("saving a copy changed the draft's origin: %+v", again.Origin)
	}
}
