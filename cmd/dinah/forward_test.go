package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/programs"
)

// The program a command is handed to is run with the command and its
// arguments, and its status is passed on. The test's own program stands in
// for it.
func TestAForwardedCommandRunsTheOtherProgramAndPassesItsStatusOn(t *testing.T) {
	dir := t.TempDir()
	if code := forwardFrom(dir, "other-program", []string{"update", "check"}); code != 1 {
		t.Fatalf("a program that is not installed gave status %d", code)
	}
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go program is not on the path")
	}
	// The go program is a real program whose status is known: "version"
	// ends with 0 and an unknown command with 2.
	data, err := os.ReadFile(gobin)
	if err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(dir, programs.FileName("other-program")), data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOROOT", goRoot(t, gobin))
	if code := forwardFrom(dir, "other-program", []string{"version"}); code != 0 {
		t.Fatalf("a command that succeeds gave status %d", code)
	}
	if code := forwardFrom(dir, "other-program", []string{"no-such-command"}); code != 2 {
		t.Fatalf("a command that fails with 2 gave status %d", code)
	}
}

func goRoot(t *testing.T, gobin string) string {
	out, err := exec.Command(gobin, "env", "GOROOT").Output()
	if err != nil {
		t.Skip(err)
	}
	return strings.TrimSpace(string(out))
}
