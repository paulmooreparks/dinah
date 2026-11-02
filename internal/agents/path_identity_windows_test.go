package agents

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
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

func TestWindowsManagedRootAllowsShortNames(t *testing.T) {
	longRoot := filepath.Join(t.TempDir(), "managed-workspace-long-name")
	if err := os.MkdirAll(longRoot, 0700); err != nil {
		t.Fatal(err)
	}
	wide, err := windows.UTF16PtrFromString(longRoot)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(wide, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || n >= uint32(len(buf)) {
		t.Fatalf("short path fixture: %d %v", n, err)
	}
	shortRoot := windows.UTF16ToString(buf[:n])
	if strings.EqualFold(longRoot, shortRoot) {
		t.Fatal("this filesystem did not supply an 8.3 short name for the required guard")
	}
	l := layout.Layout{State: shortRoot}
	managed := l.ManagedRoot("workspace")
	if err := os.MkdirAll(managed, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(managed, "nested", "draft.txt")
	if err := WriteTextFile(l, path, "legitimate", managed); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTextFile(context.Background(), l, path, []string{managed}, nil, nil)
	if err != nil || got != "legitimate" {
		t.Fatalf("managed read: %q %v", got, err)
	}
	if err := WriteTextFile(l, filepath.Join(shortRoot, "settings.json"), "forbidden", managed); err == nil {
		t.Fatal("state file became writable")
	}
	target := filepath.Join(longRoot, "redirect-target")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	redirected := l.ManagedRoot("redirected")
	if err := os.MkdirAll(filepath.Dir(redirected), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", redirected, target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("junction fixture: %s %v", out, err)
	}
	if err := WriteTextFile(l, filepath.Join(redirected, "draft.txt"), "forbidden", redirected); err == nil {
		t.Fatal("redirected managed root became writable")
	}
}
