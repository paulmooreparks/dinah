//go:build windows

package resident

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// substDrive maps a drive letter that GetLogicalDrives reports unused, from
// Z: downwards, onto target with subst, and removes that mapping, and only
// that one, when the test ends. It skips the test with the reason when no
// letter is free or subst is refused.
func substDrive(t *testing.T, target string) string {
	t.Helper()
	used, err := windows.GetLogicalDrives()
	if err != nil {
		t.Skipf("GetLogicalDrives failed: %v", err)
	}
	letter := ""
	for c := 'Z'; c >= 'D'; c-- {
		if used&(1<<uint(c-'A')) == 0 {
			letter = string(c) + ":"
			break
		}
	}
	if letter == "" {
		t.Skip("no drive letter is free for subst")
	}
	if out, err := exec.Command("subst", letter, target).CombinedOutput(); err != nil {
		t.Skipf("subst %s %s was refused: %v %s", letter, target, err, strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() {
		if out, err := exec.Command("subst", letter, "/D").CombinedOutput(); err != nil {
			t.Errorf("remove the subst drive %s: %v %s", letter, err, strings.TrimSpace(string(out)))
		}
	})
	return letter
}

// bareVolumeRoot fails the test unless a directory's own final path, read
// with VOLUME_NAME_GUID, is a volume GUID path with nothing after it.
func bareVolumeRoot(t *testing.T, what, dir string) {
	t.Helper()
	guidFinal, err := finalPathOf(openQuery(t, dir), volumeNameGUID|fileNameNormalized)
	if err != nil {
		t.Fatalf("%s: the final path of %s: %v", what, dir, err)
	}
	if _, rest, ok := volumeRootOf(guidFinal); !ok || rest != "" {
		t.Errorf("%s: WorkingDirectory answered %s, whose final path %s is not a bare volume GUID path", what, dir, guidFinal)
	}
}

// openQuery opens a path for a query and closes it when the test ends.
func openQuery(t *testing.T, path string) windows.Handle {
	t.Helper()
	handle, err := openForQuery(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { windows.CloseHandle(handle) })
	return handle
}

// driveRootOf answers the root of the drive a path's final path names.
func driveRootOf(t *testing.T, path string) string {
	t.Helper()
	dosFinal, err := finalPathOf(openQuery(t, path), volumeNameDOS|fileNameNormalized)
	if err != nil {
		t.Fatal(err)
	}
	drive, _, ok := dosDriveOf(dosFinal)
	if !ok {
		t.Fatalf("the final path %s names no drive", dosFinal)
	}
	return drive + `\`
}

// TestTheWorkingDirectoryIsAVolumeRoot is part of dinah-619/criteria/16.
// Three cases, each asserting that the answer's own final path is a bare
// volume GUID path. A workbench under a temporary directory answers the drive
// root of that directory's final path. On a subst drive, a root typed as
// X:\h\wb answers the drive root of the temporary directory's final path and
// not X:\. And with the query of the workbench's final path failing, the
// answer is the drive root of the Windows directory.
//
// Arming: answering filepath.VolumeName(root) and a backslash answers X:\ in
// the second case, whose final path carries elements after the volume root.
func TestTheWorkingDirectoryIsAVolumeRoot(t *testing.T) {
	tmp := t.TempDir()
	wb := filepath.Join(tmp, "wb")
	if err := os.MkdirAll(wb, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := WorkingDirectory(wb)
	if err != nil {
		t.Fatal(err)
	}
	if want := driveRootOf(t, tmp); !strings.EqualFold(got, want) {
		t.Errorf("a workbench under %s: WorkingDirectory answered %s, wanted %s", tmp, got, want)
	}
	bareVolumeRoot(t, "a temporary workbench", got)

	t.Run("through a subst drive", func(t *testing.T) {
		target := filepath.Join(tmp, "p", "g")
		if err := os.MkdirAll(filepath.Join(target, "h", "wb"), 0o755); err != nil {
			t.Fatal(err)
		}
		letter := substDrive(t, target)
		typed := letter + `\h\wb`
		got, err := WorkingDirectory(typed)
		if err != nil {
			t.Fatal(err)
		}
		if strings.EqualFold(got, letter+`\`) {
			t.Errorf("a root typed as %s answered %s, the subst letter's root, which is the folder %s", typed, got, target)
		}
		if want := driveRootOf(t, tmp); !strings.EqualFold(got, want) {
			t.Errorf("a root typed as %s answered %s, wanted the drive root of the temporary directory's final path, %s", typed, got, want)
		}
		bareVolumeRoot(t, "a subst drive", got)
	})

	t.Run("when the workbench's final path cannot be read", func(t *testing.T) {
		previous := finalPath
		finalPath = func(path string, flags uint32) (string, error) {
			if path == wb {
				return "", errors.New("the query of the workbench failed")
			}
			return previous(path, flags)
		}
		t.Cleanup(func() { finalPath = previous })
		got, err := WorkingDirectory(wb)
		if err != nil {
			t.Fatal(err)
		}
		windowsDir, err := windows.GetSystemWindowsDirectory()
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.VolumeName(windowsDir) + `\`; !strings.EqualFold(got, want) {
			t.Errorf("with the workbench's query failing WorkingDirectory answered %s, wanted the Windows directory's drive root %s", got, want)
		}
		bareVolumeRoot(t, "the Windows directory's drive", got)
	})
}
