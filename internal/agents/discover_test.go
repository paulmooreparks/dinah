package agents

import (
	"os"
	"path/filepath"
	"testing"
)

// Executables in a workspace are never found for it.
func TestFindExecutableSkipsWorkspaces(t *testing.T) {
	dir := t.TempDir()
	name := "fake-agent"
	exe := filepath.Join(dir, name+".exe")
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findExecutableIn(name, nil, dir); got == "" {
		t.Fatal("an executable on the search path was not found")
	}
	if got := findExecutableIn(name, []string{dir}, dir); got != "" {
		t.Fatalf("an executable inside a workspace was found: %s", got)
	}
}
