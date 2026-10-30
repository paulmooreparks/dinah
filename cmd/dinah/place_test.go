package main

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
	if err := place(dst, dst); err != nil {
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
	if err := place(src, dst); err != nil {
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
	if err := place(src, dst); err == nil {
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
	if err := place(filepath.Join(dir, "missing.exe"), dst); err == nil {
		t.Fatal("expected an error")
	}
	if got := readProgram(t, dst); got != "old" {
		t.Fatalf("the installed program changed: %q", got)
	}
}
