package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// A file the page made is written whole to the place chosen, replacing
// what was there, and a path that is not a full one is refused.
func TestAFileThePageMadeIsWrittenToTheComputer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screen.png")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	content, err := computerFileBytes(base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G', 0, 1, 2}))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeComputerFile(path, content); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(content) {
		t.Fatalf("the file holds %q", got)
	}
	left, _ := os.ReadDir(dir)
	if len(left) != 1 {
		t.Fatalf("the folder has %d files after the write", len(left))
	}
	if err := writeComputerFile("screen.png", content); err == nil {
		t.Fatal("a path that is not a full one was written")
	}
	if _, err := computerFileBytes("not base64!"); err == nil {
		t.Fatal("text that is not base64 was taken")
	}
}

func TestTheSaveDialogOffersTheFilesKindFirst(t *testing.T) {
	f := computerFileFilters("Payment Cash.png")
	if len(f) != 2 || f[0].Pattern != "*.png" || f[0].DisplayName != "PNG Files (*.png)" || f[1].Pattern != "*.*" {
		t.Fatalf("filters %+v", f)
	}
	if f := computerFileFilters("notes"); len(f) != 1 || f[0].Pattern != "*.*" {
		t.Fatalf("filters for a name with no extension %+v", f)
	}
}
