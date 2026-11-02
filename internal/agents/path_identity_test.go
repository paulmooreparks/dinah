package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/layout"
)

func TestProtectedIdentityWritesAndManagedException(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	l := layout.Layout{State: state}
	managed := l.ManagedRoot("workspace")
	if err := os.MkdirAll(managed, 0700); err != nil {
		t.Fatal(err)
	}
	ordinary := filepath.Join(t.TempDir(), "nested", "file.txt")
	for _, text := range []string{"long original text", "new"} {
		if err := WriteTextFile(l, ordinary, text); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(ordinary); err != nil || string(got) != text {
			t.Fatalf("ordinary write: %q %v", got, err)
		}
	}
	protected := filepath.Join(state, "settings.json")
	if err := os.WriteFile(protected, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteTextFile(l, protected, "bad"); err == nil {
		t.Fatal("protected write succeeded")
	}
	if got, _ := os.ReadFile(protected); string(got) != "original" {
		t.Fatal("refusal truncated original")
	}
	path := filepath.Join(managed, "nested", "draft.txt")
	if err := WriteTextFile(l, path, "managed", managed); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadTextFile(context.Background(), l, path, []string{managed}, nil, nil); err != nil || got != "managed" {
		t.Fatalf("managed read: %q %v", got, err)
	}
	redirect := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(state, redirect); err != nil {
		t.Fatal(err)
	}
	if err := WriteTextFile(l, filepath.Join(redirect, "settings.json"), "bad"); err == nil {
		t.Fatal("alias write reached state")
	}
	if err := WriteTextFile(l, filepath.Join(redirect, "new", "file.txt"), "bad"); err == nil {
		t.Fatal("alias created a protected descendant")
	}
	if _, err := os.Stat(filepath.Join(state, "new")); !os.IsNotExist(err) {
		t.Fatal("refusal created a protected directory")
	}
}

func TestProtectedIdentityCaseVariant(t *testing.T) {
	state := filepath.Join(t.TempDir(), "MixedCaseState")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(state, "Settings.txt")
	if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(state), "mixedcasestate", "settings.txt")
	info, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("this filesystem has distinct case variants")
	}
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(path)
	if err != nil || !os.SameFile(original, info) {
		t.Fatal("case fixture is not an identity alias")
	}
	l := layout.Layout{State: state}
	if ProtectedPath(l, alias) == "" {
		t.Fatal("case alias was unprotected")
	}
	if _, err := ReadTextFile(context.Background(), l, alias, nil, nil, nil); err == nil {
		t.Fatal("read accepted case alias")
	}
	if err := WriteTextFile(l, alias, "bad"); err == nil {
		t.Fatal("write accepted case alias")
	}
	// Exercise the final held-handle check independently of the early check.
	canonical, err := ResolveFilePath(alias)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.VolumeName(canonical) + string(filepath.Separator)
	rel, _ := filepath.Rel(base, canonical)
	file, err := openReadFileChecked(base, rel, func(chain []os.FileInfo) error { return protectedIdentity(l, chain) })
	if file != nil {
		file.Close()
	}
	if err == nil {
		t.Fatal("held identities failed to deny case alias")
	}
	if got, _ := os.ReadFile(path); strings.TrimSpace(string(got)) != "synthetic" {
		t.Fatal("case write changed original")
	}
}
