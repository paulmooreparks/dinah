package agents

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"studio/internal/layout"
)

func TestWindowsProtectedPathNamespaceReadAndWrite(t *testing.T) {
	state := t.TempDir()
	path := filepath.Join(state, "settings.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	l := layout.Layout{State: state}
	share := `\\localhost\` + string(path[0]) + `$` + path[2:]
	variants := []string{path, `\\?\` + path, `\\.\` + path, share, `\\?\UNC\` + strings.TrimPrefix(share, `\\`), `\\?\GLOBALROOT\Device\HarddiskVolume1\settings.json`, `\\?\Volume{synthetic}\settings.json`}
	for _, alias := range variants {
		t.Run(alias, func(t *testing.T) {
			if ProtectedPath(l, alias) == "" {
				t.Fatal("alias was unprotected")
			}
			if _, err := ReadTextFile(context.Background(), l, alias, nil, nil, nil); err == nil {
				t.Fatal("read accepted alias")
			}
			if err := WriteTextFile(l, alias, "bad"); err == nil {
				t.Fatal("write accepted alias")
			}
		})
	}
	if got, _ := os.ReadFile(path); string(got) != "original" {
		t.Fatal("a refusal changed the protected file")
	}
	ordinary := filepath.Join(t.TempDir(), "allowed.txt")
	extended := `\\?\` + ordinary
	if got, err := NormalizeFilePath(extended); err != nil || got != ordinary {
		t.Fatalf("normalization: %q %v", got, err)
	}
	if err := WriteTextFile(l, extended, "allowed"); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadTextFile(context.Background(), l, extended, nil, nil, nil); err != nil || got != "allowed" {
		t.Fatalf("extended allowed: %q %v", got, err)
	}
}

func TestWindowsJunctionWriteCannotReachState(t *testing.T) {
	state := t.TempDir()
	path := filepath.Join(state, "settings.json")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "junction")
	cmd := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", link, state)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("junction fixture: %s %v", out, err)
	}
	l := layout.Layout{State: state}
	if err := WriteTextFile(l, filepath.Join(link, "settings.json"), "bad"); err == nil {
		t.Fatal("junction write reached state")
	}
	if got, _ := os.ReadFile(path); string(got) != "original" {
		t.Fatal("junction write changed protected data")
	}
}
