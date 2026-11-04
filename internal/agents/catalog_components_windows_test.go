package agents

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsCatalogTagDecision(t *testing.T) {
	// These WinNT.h tag values distinguish name surrogates from storage filters.
	for _, tc := range []struct {
		name    string
		tag     uint32
		refused bool
	}{
		{"SYMLINK", 0xA000000C, true},
		{"MOUNT_POINT", 0xA0000003, true},
		{"CLOUD", 0x9000001A, false},
		{"DEDUP", 0x80000013, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := catalogNameSurrogate(windows.FILE_ATTRIBUTE_REPARSE_POINT, tc.tag); got != tc.refused {
				t.Fatalf("tag %#x: refusal %v, want %v", tc.tag, got, tc.refused)
			}
		})
	}
	if catalogNameSurrogate(windows.FILE_ATTRIBUTE_NORMAL, 0xA000000C) {
		t.Fatal("a non-reparse file's unused tag was interpreted")
	}
}

func TestWindowsCatalogRejectsLinks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ordinary.txt")
	if err := os.WriteFile(path, []byte("synthetic catalogue"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := OpenCatalogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	link := filepath.Join(t.TempDir(), "file-link.txt")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(t.TempDir(), "junction")
	command := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junction, root)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("junction fixture: %s %v", out, err)
	}
	for _, path := range []string{link, filepath.Join(junction, "ordinary.txt")} {
		if file, err := OpenCatalogFile(path); err == nil {
			file.Close()
			t.Fatalf("catalogue followed a link: %s", path)
		}
	}
}
