package localfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/resource"
)

// CreateFolder makes a folder inside the source, refuses a taken name, a
// name no system allows, a hidden path and a missing parent.
func TestCreateFolder(t *testing.T) {
	root := t.TempDir()
	f := Folder{Root: root}
	if err := f.CreateFolder("Note 12"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(root, "Note 12")); err != nil || !st.IsDir() {
		t.Fatalf("not made: %v", err)
	}
	if err := f.CreateFolder("Note 12/research"); err != nil {
		t.Fatal(err)
	}
	if err := f.CreateFolder("Note 12"); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("a taken name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.CreateFolder("notes.md"); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("a file's name: %v", err)
	}
	for _, bad := range []string{"a:b", "what?", "CON", "nul.txt", "trailing.", " space", ".hidden", "../out", "missing/child"} {
		if err := f.CreateFolder(bad); err == nil {
			t.Errorf("%q was made", bad)
		}
	}
	if err := (Folder{Root: root, Dotfiles: true}).CreateFolder(".config"); err != nil {
		t.Fatalf("a home folder takes a dot name: %v", err)
	}
}
