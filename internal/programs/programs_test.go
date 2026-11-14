package programs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeProgram(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readProgram(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s is missing: %v", path, err)
	}
	return string(b)
}

func TestPlaceFromTheInstalledProgramItself(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dinah.exe")
	writeProgram(t, dst, "the installed program")
	if err := Place(dst, dst); err != nil {
		t.Fatalf("installing from the installed program failed: %v", err)
	}
	if got := readProgram(t, dst); got != "the installed program" {
		t.Fatalf("the program changed: %q", got)
	}
}

func TestPlaceReplacesAndSetsTheOldOneAside(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.exe"), filepath.Join(dir, "dinah.exe")
	writeProgram(t, src, "new")
	writeProgram(t, dst, "old")
	if err := Place(src, dst); err != nil {
		t.Fatal(err)
	}
	if got := readProgram(t, dst); got != "new" {
		t.Fatalf("installed program is %q", got)
	}
}

func TestPlaceRestoresTheOldProgramWhenTheLastStepFails(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.exe"), filepath.Join(dir, "dinah.exe")
	writeProgram(t, src, "new")
	writeProgram(t, dst, "old")
	real := renameFile
	defer func() { renameFile = real }()
	renameFile = func(from, to string) error {
		if to == dst && filepath.Ext(from) == ".new" {
			return errors.New("refused")
		}
		return real(from, to)
	}
	if err := Place(src, dst); err == nil {
		t.Fatal("expected the failure to be reported")
	}
	if got := readProgram(t, dst); got != "old" {
		t.Fatalf("the old program was not put back: %q", got)
	}
	if _, err := os.Stat(dst + ".new"); err == nil {
		t.Fatal("the half-installed copy was left behind")
	}
}

func TestPlaceLeavesTheInstalledProgramWhenTheSourceCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dinah.exe")
	writeProgram(t, dst, "old")
	if err := Place(filepath.Join(dir, "missing.exe"), dst); err == nil {
		t.Fatal("expected an error")
	}
	if got := readProgram(t, dst); got != "old" {
		t.Fatalf("the installed program changed: %q", got)
	}
}

func TestPlaceKeepsAllSetAsideProgramsAcrossInstallsAndFailures(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.exe"), filepath.Join(dir, "window.exe")
	// Retention does not depend on file locks, process visibility or clock order.
	kept := map[string]string{dst + ".old-100": "a running earlier build", dst + ".old-900": "the latest earlier build"}
	for path, body := range kept {
		writeProgram(t, path, body)
	}
	writeProgram(t, dst, "installed before first replacement")
	for _, body := range []string{"replacement one", "replacement two"} {
		writeProgram(t, src, body)
		if err := Place(src, dst); err != nil {
			t.Fatal(err)
		}
		files, err := filepath.Glob(dst + ".old-*")
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			data := readProgram(t, path)
			if previous, exists := kept[path]; exists && previous != data {
				t.Fatalf("fallback overwritten: %s", path)
			}
			kept[path] = data
		}
	}
	if len(kept) != 4 {
		t.Fatalf("set-aside programs: %v", kept)
	}
	// A fallback can itself be the install source and must remain available.
	if err := Place(dst+".old-100", dst); err != nil {
		t.Fatal(err)
	}
	if err := Place(filepath.Join(dir, "missing.exe"), dst); err == nil {
		t.Fatal("missing source accepted")
	}
	for path, body := range kept {
		if got := readProgram(t, path); got != body {
			t.Fatalf("fallback %s changed to %q", path, got)
		}
	}
}

func TestStartOption(t *testing.T) {
	if _, ok := StartOption("--no-such-option"); ok {
		t.Fatal("an option nobody registered was found")
	}
	var got []string
	RegisterStartOption("--test-option", func(args []string) int {
		got = args
		return 3
	})
	run, ok := StartOption("--test-option")
	if !ok {
		t.Fatal("the registered option was not found")
	}
	if status := run([]string{"a", "b"}); status != 3 || len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("the option ran with %q and returned %d", got, status)
	}
}
