//go:build unix

package bench

import (
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/durable"
)

// TestARenameIntoPlaceSurvivesAForeignReader is the Windows test's body on a
// platform whose rename(2) is not refused by another process's open handle:
// the write lands the new bytes over an open reader on its first attempt,
// with no wait, and leaves no temporary.
func TestARenameIntoPlaceSurvivesAForeignReader(t *testing.T) {
	root := newDurableFixture(t)
	path := filepath.Join(fixtureCardDir(root), CardAnchor)
	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("open a foreign reader: %v", err)
	}
	defer reader.Close()
	replaces := 0
	durable.Observe = func(step durable.Step) error {
		if step.Op == "replace" {
			replaces++
		}
		return nil
	}
	t.Cleanup(func() { durable.Observe = nil })
	if err := WriteText(path, "new bytes\n"); err != nil {
		t.Fatalf("a write over a foreign reader answered %v", err)
	}
	if replaces != 1 {
		t.Errorf("the rename was attempted %d times, wanted once", replaces)
	}
	if text, _ := ReadText(path); text != "new bytes\n" {
		t.Errorf("the file holds %q after the write", text)
	}
	if left := temporariesIn(t, filepath.Dir(path)); len(left) != 0 {
		t.Errorf("temporaries stand: %v", left)
	}
}
