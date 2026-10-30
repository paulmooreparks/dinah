package localfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/resource"
)

// A document's image is read by its path from the document's folder, as a
// README shows src/static/logo.png, or by its name beside it; a path never
// leaves the source.
func TestReadFileByPath(t *testing.T) {
	outer := t.TempDir()
	dir := filepath.Join(outer, "repo")
	os.MkdirAll(filepath.Join(dir, "src", "static"), 0o755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# R\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "src", "static", "logo.png"), []byte("logo"), 0o644)
	os.WriteFile(filepath.Join(dir, "beside.png"), []byte("beside"), 0o644)
	os.WriteFile(filepath.Join(outer, "secret.png"), []byte("secret"), 0o644)
	f := Folder{Root: dir}
	ctx := context.Background()
	for name, want := range map[string]string{"src/static/logo.png": "logo", "beside.png": "beside", "elsewhere/beside.png": "beside"} {
		if data, err := f.ReadFile(ctx, "README.md", name); err != nil || string(data) != want {
			t.Errorf("%s: %q %v", name, data, err)
		}
	}
	for _, name := range []string{"../secret.png", "/secret.png", `..\secret.png`, "C:/secret.png"} {
		if data, err := f.ReadFile(ctx, "README.md", name); err == nil {
			t.Errorf("%s was read from outside the source: %q", name, data)
		}
	}
}

func TestReadWrite(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "notes"), 0o755)
	os.WriteFile(filepath.Join(dir, "notes", "a.md"), []byte("# A\n"), 0o644)
	f := Folder{Root: dir}
	ctx := context.Background()

	got, err := f.ReadText(ctx, "notes/a.md")
	if err != nil || got.Text != "# A\n" || got.Format != "markdown" || got.Revision != resource.ContentRevision("# A\n") {
		t.Fatalf("read %+v %v", got, err)
	}
	rev, err := f.WriteText(ctx, "notes/a.md", got.Revision, "# A\n\nMore.\n")
	if err != nil || rev != resource.ContentRevision("# A\n\nMore.\n") {
		t.Fatalf("write %q %v", rev, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "notes", "a.md"))
	if string(data) != "# A\n\nMore.\n" {
		t.Fatalf("on disk %q", data)
	}
	// The old revision is now stale.
	if _, err := f.WriteText(ctx, "notes/a.md", got.Revision, "lost"); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("stale write: %v", err)
	}
	// A change made outside the app is detected.
	os.WriteFile(filepath.Join(dir, "notes", "a.md"), []byte("edited elsewhere"), 0o644)
	if _, err := f.WriteText(ctx, "notes/a.md", rev, "mine"); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("outside change was overwritten: %v", err)
	}
	// No temporary files are left behind.
	entries, _ := os.ReadDir(filepath.Join(dir, "notes"))
	if len(entries) != 1 {
		t.Fatalf("left %d entries", len(entries))
	}
}

func TestCreate(t *testing.T) {
	dir := t.TempDir()
	f := Folder{Root: dir}
	ctx := context.Background()
	if _, err := f.WriteText(ctx, "new.md", "", "fresh"); err != nil {
		t.Fatal(err)
	}
	// Creating over an existing file, or naming a revision for a file that
	// does not exist, is a conflict.
	if _, err := f.WriteText(ctx, "new.md", "", "again"); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("created over an existing file: %v", err)
	}
	if _, err := f.WriteText(ctx, "absent.md", "abc", "x"); !errors.Is(err, resource.ErrConflict) {
		t.Fatalf("a revision for an absent file: %v", err)
	}
	if _, err := f.WriteText(ctx, "missing/dir.md", "", "x"); err == nil {
		t.Fatal("wrote into a folder that does not exist")
	}
	if _, err := f.ReadText(ctx, "absent.md"); !errors.Is(err, resource.ErrNotFound) {
		t.Fatalf("absent read: %v", err)
	}
}

func TestRefusals(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.md"), []byte("s"), 0o644)
	f := Folder{Root: dir}
	ctx := context.Background()
	for _, p := range []string{"../x.md", ".hidden.md", "a/../b.md", "/etc/passwd", "C:/x.md", "a\\b.md", " a.md", "", "."} {
		if _, err := f.WriteText(ctx, p, "", "x"); err == nil {
			t.Errorf("wrote %q", p)
		}
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err == nil {
		if _, err := f.ReadText(ctx, "link/secret.md"); err == nil {
			t.Error("read through a symbolic link")
		}
		if _, err := f.WriteText(ctx, "link/new.md", "", "x"); err == nil {
			t.Error("wrote through a symbolic link")
		}
	}
	big := strings.Repeat("x", MaxText+1)
	if _, err := f.WriteText(ctx, "big.md", "", big); err == nil {
		t.Error("wrote more than the limit")
	}
	if _, err := f.WriteText(ctx, "nul.md", "", "a\x00b"); err == nil {
		t.Error("wrote a NUL character")
	}
	os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{0xff, 0xfe, 0x00}, 0o644)
	if _, err := f.ReadText(ctx, "bin.dat"); err == nil {
		t.Error("read a binary file as text")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.WriteText(cancelled, "c.md", "", "x"); err == nil {
		t.Error("wrote after cancellation")
	}
}
