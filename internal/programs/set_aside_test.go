package programs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetAsideDatesAndFallbackPrecedence(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"window" + ".old-20261113120000000",
		"window" + ".old-20261114120000000-a",
		"window" + ".old-20261114120000000-b",
		"window" + ".before-manual-update-20261114-150000",
		"command" + ".before-first-20261113-120000",
		"command" + ".before-second-20261114-120000",
		"window" + ".unrelated",
	} {
		parts := strings.SplitN(name, ".", 2)
		writeProgram(t, filepath.Join(dir, FileName(parts[0])+"."+parts[1]), name)
	}
	service := NewSetAsideService(dir, []string{"window", "command", "window"})
	files, err := service.scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 6 {
		t.Fatalf("got %d files, want 6", len(files))
	}
	for _, file := range files {
		wantFallback := strings.Contains(file.Name, ".old-20261114120000000") || strings.Contains(file.Name, ".before-second-")
		if file.Fallback != wantFallback || file.CanRemove == wantFallback {
			t.Fatalf("wrong protection: %+v", file)
		}
	}
	stamp, old, valid, match := asideDate(FileName("window")+".old-1794648000000000000-123", "window")
	if !old || !valid || !match || stamp.UnixNano() != 1794648000000000000 {
		t.Fatalf("nanosecond suffix: %v %v %v %v", stamp, old, valid, match)
	}
}

func TestSetAsideInvalidDatesPreventGuessingFallback(t *testing.T) {
	dir := t.TempDir()
	for _, suffix := range []string{".old-nonsense", ".old-20261113120000000", ".before-change-20260230-120000"} {
		writeProgram(t, filepath.Join(dir, FileName("window")+suffix), suffix)
	}
	files, err := NewSetAsideService(dir, []string{"window"}).scan()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.CanRemove {
			t.Fatalf("unrecognized date allowed deletion: %+v", file)
		}
	}
}

func TestSetAsideIDsChangeWithIdentitySizeAndModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName("window")+".old-20261113120000000")
	writeProgram(t, path, "first")
	service := NewSetAsideService(dir, []string{"window"})
	first, err := service.scan()
	if err != nil {
		t.Fatal(err)
	}
	next := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, next, next); err != nil {
		t.Fatal(err)
	}
	second, err := service.scan()
	if err != nil || first[0].ID == second[0].ID {
		t.Fatalf("modified file kept its selection: %v", err)
	}
	if err := os.Rename(path, path+".kept"); err != nil {
		t.Fatal(err)
	}
	writeProgram(t, path, "first")
	third, err := service.scan()
	if err != nil || first[0].ID == third[0].ID {
		t.Fatalf("replacement kept its selection: %v", err)
	}
}

func TestSetAsideRejectsMalformedRemovalBeforeChanges(t *testing.T) {
	service := NewSetAsideService(t.TempDir(), []string{"window"})
	for _, ids := range [][]string{nil, {"../window"}, {strings.Repeat("0", 64), strings.Repeat("0", 64)}, {strings.Repeat("z", 64)}, make([]string, 101)} {
		if _, err := service.Remove(RemoveSetAsideInput{IDs: ids}); err == nil {
			t.Fatalf("accepted malformed selection: %v", ids)
		}
	}
}
