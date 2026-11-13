package diagram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A diagram imports nothing, so one written by someone else cannot read a
// file of the person's into a picture that is shown or published. Without
// the empty file system d2 opens an import from the working folder, and the
// renderer did so until 13 November 2026.
func TestRenderRefusesImports(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.d2"), []byte("leaked: TOP-SECRET-VALUE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	svg, err := Render("x: @secret\n")
	if err == nil && strings.Contains(string(svg), "TOP-SECRET-VALUE") {
		t.Fatal("the imported file's content was drawn")
	}
	if err == nil || !strings.Contains(err.Error(), "failed to import") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("an import was not refused as an import: %v", err)
	}
}
