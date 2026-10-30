package localfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	os.MkdirAll(filepath.Dir(full), 0o755)
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(dir, name string) bool {
	_, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil
}

func TestMakeFolder(t *testing.T) {
	dir := t.TempDir()
	f, ctx := Folder{Root: dir}, context.Background()
	if err := f.MakeFolder(ctx, "a", false); err != nil || !exists(dir, "a") {
		t.Fatalf("mkdir: %v", err)
	}
	if err := f.MakeFolder(ctx, "a", false); !errors.Is(err, ErrExists) {
		t.Fatalf("an existing folder: %v", err)
	}
	if err := f.MakeFolder(ctx, "b/c", false); err == nil {
		t.Fatal("a folder whose parent is missing was made")
	}
	if err := f.MakeFolder(ctx, "b/c", true); err != nil || !exists(dir, "b/c") {
		t.Fatalf("mkdir -p: %v", err)
	}
	if err := f.MakeFolder(ctx, "b/c", true); err != nil {
		t.Fatalf("mkdir -p on an existing folder: %v", err)
	}
	if err := f.MakeFolder(ctx, ".git", true); err == nil {
		t.Fatal("a hidden folder was made")
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	f, ctx := Folder{Root: dir}, context.Background()
	write(t, dir, "a.txt", "a")
	write(t, dir, "tree/x.txt", "x")
	write(t, dir, "tree/sub/y.txt", "y")
	os.Mkdir(filepath.Join(dir, "empty"), 0o755)

	if n, err := f.Remove(ctx, "a.txt", false); err != nil || n != 1 || exists(dir, "a.txt") {
		t.Fatalf("rm a file: %d %v", n, err)
	}
	if _, err := f.Remove(ctx, "a.txt", false); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rm a missing file: %v", err)
	}
	if n, err := f.Remove(ctx, "empty", false); err != nil || n != 1 {
		t.Fatalf("rmdir: %d %v", n, err)
	}
	if _, err := f.Remove(ctx, "tree", false); err == nil || !exists(dir, "tree/x.txt") {
		t.Fatalf("a full folder was removed without -r: %v", err)
	}
	if n, err := f.Remove(ctx, "tree", true); err != nil || n != 4 || exists(dir, "tree") {
		t.Fatalf("rm -r: %d %v", n, err)
	}
	if _, err := f.Remove(ctx, "", true); err == nil || !strings.Contains(err.Error(), "own folder") {
		t.Fatalf("the root was removed: %v", err)
	}
	// A tree holding something the person cannot see is left alone.
	write(t, dir, "repo/readme.md", "r")
	write(t, dir, "repo/.git/HEAD", "ref")
	if _, err := f.Remove(ctx, "repo", true); err == nil || !strings.Contains(err.Error(), "hidden") || !exists(dir, "repo/.git/HEAD") {
		t.Fatalf("a tree with hidden entries was removed: %v", err)
	}
}

func TestMove(t *testing.T) {
	dir := t.TempDir()
	f, ctx := Folder{Root: dir}, context.Background()
	write(t, dir, "a.txt", "a")
	write(t, dir, "b.txt", "b")
	write(t, dir, "d/x.txt", "x")
	os.Mkdir(filepath.Join(dir, "e"), 0o755)

	if err := f.Move(ctx, "a.txt", "c.txt"); err != nil || exists(dir, "a.txt") || !exists(dir, "c.txt") {
		t.Fatalf("rename: %v", err)
	}
	if err := f.Move(ctx, "c.txt", "b.txt"); err != nil {
		t.Fatalf("a file replacing a file: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "b.txt")); string(data) != "a" {
		t.Fatalf("the replaced file holds %q", data)
	}
	if err := f.Move(ctx, "d", "e"); !errors.Is(err, ErrExists) {
		t.Fatalf("a folder replacing a folder: %v", err)
	}
	if err := f.Move(ctx, "d", "d/inner"); err == nil {
		t.Fatal("a folder moved into itself")
	}
	if err := f.Move(ctx, "d", "e/d"); err != nil || !exists(dir, "e/d/x.txt") {
		t.Fatalf("a folder into another: %v", err)
	}
	if err := f.Move(ctx, "", "z"); err == nil {
		t.Fatal("the root was moved")
	}
}

func TestCopy(t *testing.T) {
	dir, other := t.TempDir(), t.TempDir()
	f, g, ctx := Folder{Root: dir}, Folder{Root: other}, context.Background()
	write(t, dir, "a.txt", "a")
	write(t, dir, "d/x.txt", "x")
	write(t, dir, "d/sub/y.bin", "\x00\x01binary")

	if n, err := Copy(ctx, f, "a.txt", f, "b.txt", false); err != nil || n != 1 || !exists(dir, "b.txt") || !exists(dir, "a.txt") {
		t.Fatalf("cp: %d %v", n, err)
	}
	if _, err := Copy(ctx, f, "d", f, "e", false); err == nil {
		t.Fatal("a folder was copied without -r")
	}
	if n, err := Copy(ctx, f, "d", g, "d", true); err != nil || n != 4 {
		t.Fatalf("cp -r to another folder: %d %v", n, err)
	}
	if data, _ := os.ReadFile(filepath.Join(other, "d", "sub", "y.bin")); string(data) != "\x00\x01binary" {
		t.Fatalf("binary copy %q", data)
	}
	if _, err := Copy(ctx, f, "d", g, "d", true); !errors.Is(err, ErrExists) {
		t.Fatalf("a folder copied over a folder: %v", err)
	}
	if _, err := Copy(ctx, f, "d", f, "d/inner", true); err == nil {
		t.Fatal("a folder copied into itself")
	}
	write(t, dir, "h/.secret", "s")
	if _, err := Copy(ctx, f, "h", g, "h", true); err == nil || exists(other, "h") {
		t.Fatalf("a tree with hidden entries was copied: %v", err)
	}
}
