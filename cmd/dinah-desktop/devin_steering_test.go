package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A command program beside the window is found without PATH, and its folder
// goes first on the PATH a Devin chat is started with.
func TestCommandProgramBesideTheWindowNeedsNoPath(t *testing.T) {
	dir := t.TempDir()
	name := product.CommandProgram
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	old := devinSteeringExecutable
	devinSteeringExecutable = func() (string, error) { return filepath.Join(dir, "window"), nil }
	t.Cleanup(func() { devinSteeringExecutable = old })
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	if got, err := commandProgramDir(); err == nil {
		t.Fatalf("a command that is nowhere was found in %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := commandProgramDir()
	if err != nil || got != dir {
		t.Fatalf("commandProgramDir = %q, %v; want %q", got, err, dir)
	}
	if err := ensureDevinSteeringHooks(t.TempDir()); err != nil {
		t.Fatalf("hooks with the command beside the window: %v", err)
	}
	for _, entry := range devinEnvironment() {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PATH") && !strings.HasPrefix(value, dir+string(os.PathListSeparator)) {
			t.Fatalf("PATH does not start with the command's folder: %s", value)
		}
	}
}

func TestWithPathFirst(t *testing.T) {
	sep := string(os.PathListSeparator)
	got := withPathFirst([]string{"A=1", "PATH=x" + sep + "y", "B=2"}, "d")
	if want := "PATH=d" + sep + "x" + sep + "y"; len(got) != 3 || got[1] != want {
		t.Fatalf("got %q, want the middle entry %q", got, want)
	}
	if got := withPathFirst([]string{"A=1"}, "d"); len(got) != 2 || got[1] != "PATH=d" {
		t.Fatalf("an environment with no PATH: %q", got)
	}
	if runtime.GOOS == "windows" {
		if got := withPathFirst([]string{"Path=x"}, "d"); len(got) != 1 || got[0] != "Path=d"+sep+"x" {
			t.Fatalf("Windows spelling: %q", got)
		}
	}
}

func TestDevinSteeringRegistrationPreservesHooksAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.v1.json")
	initial := `{"PostToolUse":[{"matcher":"exec","hooks":[{"type":"command","command":"my-check"}]}],"SessionStart":[{"hooks":[{"type":"command","command":"my-start"}]}]}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := registerDevinSteeringHooks(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if !strings.Contains(string(first), "my-check") || !strings.Contains(string(first), "my-start") || strings.Count(string(first), devinSteeringCommand) != 3 {
		t.Fatalf("registered hooks: %s", first)
	}
	if err := registerDevinSteeringHooks(path); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatal("registration duplicated or changed existing hooks")
	}
	broken := []byte(`{"PostToolUse":42}`)
	os.WriteFile(path, broken, 0o600)
	if err := registerDevinSteeringHooks(path); err == nil {
		t.Fatal("invalid hooks were replaced")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(broken) {
		t.Fatal("failed registration changed the person's hooks")
	}
}
