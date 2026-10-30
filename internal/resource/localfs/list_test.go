package localfs

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"studio/internal/resource"
)

func TestListAndFS(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "types", "design-note"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, "types", "design-note", "doctype.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "types", "design-note", "template.md"), []byte("# T\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "types", ".hidden.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("r"), 0o644)
	f := Folder{Root: dir}
	ctx := context.Background()

	got, err := f.List(ctx, ".")
	if err != nil || len(got) != 2 || got[0] != (resource.Entry{Name: "readme.md", Size: 1}) || got[1] != (resource.Entry{Name: "types", Folder: true}) {
		t.Fatalf("root listing %+v %v", got, err)
	}
	if _, err := f.List(ctx, "missing"); !errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("missing folder: %v", err)
	}
	if _, err := f.List(ctx, "readme.md"); err == nil {
		t.Fatal("listed a file")
	}
	if _, err := f.List(ctx, ".git"); err == nil {
		t.Fatal("listed a hidden folder")
	}

	fsys, err := resource.FS(ctx, f, "types")
	if err != nil {
		t.Fatal(err)
	}
	if err := fstest.TestFS(fsys, "design-note/doctype.json", "design-note/template.md"); err != nil {
		t.Fatal(err)
	}
	if data, err := fs.ReadFile(fsys, "design-note/template.md"); err != nil || string(data) != "# T\n" {
		t.Fatalf("%q %v", data, err)
	}
	if _, err := fs.Stat(fsys, "design-note/none.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	if _, err := resource.FS(ctx, struct{}{}, "."); !errors.Is(err, resource.ErrUnsupported) {
		t.Fatalf("a provider that cannot list: %v", err)
	}
}
