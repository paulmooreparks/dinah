package layout

import (
	"path/filepath"
	"testing"
)

func TestLayout(t *testing.T) {
	state := t.TempDir()
	ws := t.TempDir()
	l := Layout{State: state}
	if l.RunDir(ws) != filepath.Join(ws, ".dinah", "runs") {
		t.Fatal(l.RunDir(ws))
	}
	managed := l.ManagedRoot("abc")
	if !l.Managed(managed) || l.Managed(filepath.Dir(managed)) || l.Managed(ws) {
		t.Fatal("managed folders are misjudged")
	}
	if l.RunDir(managed) != filepath.Join(state, "workspaces", "abc", "runs") {
		t.Fatal(l.RunDir(managed))
	}
}
