package agents

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"studio/internal/acp"
)

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Skip("no " + from + " to copy: " + err.Error())
	}
	if err := os.WriteFile(to, b, 0o755); err != nil {
		t.Fatal(err)
	}
}

// On a machine where Devin Desktop is on the search path ahead of the Devin
// command line, and a .cmd launcher and an old command line that does not
// answer are there too, FindDevin passes each over, says why, and uses the
// command line that answers.
func TestFindDevinPassesOverWhatIsNotTheCommandLine(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the desktop and launcher cases are Windows ones")
	}
	system := filepath.Join(os.Getenv("SystemRoot"), "System32")
	desktop, launcher, old, cli := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	copyFile(t, filepath.Join(system, "notepad.exe"), filepath.Join(desktop, "Devin.exe"))
	if err := os.WriteFile(filepath.Join(launcher, "devin.cmd"), []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(system, "where.exe"), filepath.Join(old, "devin.exe"))
	copyFile(t, filepath.Join(system, "whoami.exe"), filepath.Join(cli, "devin.exe"))
	t.Setenv("PATH", strings.Join([]string{desktop, launcher, old, cli}, string(os.PathListSeparator)))
	saved, savedRelease := probeACP, releaseOf
	defer func() { probeACP, releaseOf = saved, savedRelease }()
	releaseOf = func(string) string { return "" }
	var started []string
	probeACP = func(path string) (acp.InitializeResult, error) {
		started = append(started, path)
		if strings.HasPrefix(path, cli) {
			return acp.InitializeResult{ProtocolVersion: ProtocolVersion, AgentInfo: &acp.Implementation{Name: "devin", Version: "3000.10.48"}}, nil
		}
		return acp.InitializeResult{}, errors.New("unknown command acp")
	}
	path, found := FindDevin(nil)
	if !strings.HasPrefix(path, cli) {
		t.Fatalf("found %q; candidates %+v", path, found)
	}
	// A Devin installed on this machine, on the account's own search path,
	// may follow the four; it is checked too, and here does not answer.
	if len(found) < 4 {
		t.Fatalf("candidates %+v", found)
	}
	if !strings.Contains(found[0].Problem, "desktop program") || !strings.Contains(found[1].Problem, "not an .exe") || !strings.Contains(found[2].Problem, "did not answer") || found[3].Problem != "" || found[3].Version != "3000.10.48" {
		t.Fatalf("candidates %+v", found)
	}
	// Devin Desktop and the launcher are never started.
	for _, p := range started {
		if strings.HasPrefix(p, desktop) || strings.HasPrefix(p, launcher) {
			t.Fatalf("started %q", started)
		}
	}
	// A second look starts nothing again.
	before := len(started)
	if FindDevin(nil); len(started) != before {
		t.Fatalf("started again: %q", started)
	}
}

// A stale copy that still answers, ahead of the current one on the search
// path, is not used quietly: FindDevin returns no path, both copies are the
// choices with what they print as their version, and the newer is suggested.
func TestFindDevinLeavesTwoWorkingCopiesToThePerson(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("copies of Windows console programs")
	}
	system := filepath.Join(os.Getenv("SystemRoot"), "System32")
	stale, current := t.TempDir(), t.TempDir()
	copyFile(t, filepath.Join(system, "where.exe"), filepath.Join(stale, "devin.exe"))
	copyFile(t, filepath.Join(system, "whoami.exe"), filepath.Join(current, "devin.exe"))
	t.Setenv("PATH", stale+string(os.PathListSeparator)+current)
	savedProbe, savedRelease := probeACP, releaseOf
	defer func() { probeACP, releaseOf = savedProbe, savedRelease }()
	probeACP = func(string) (acp.InitializeResult, error) {
		return acp.InitializeResult{ProtocolVersion: ProtocolVersion}, nil
	}
	releaseOf = func(path string) string {
		if strings.HasPrefix(path, stale) {
			return "devin 2026.4.15-0"
		}
		return "devin 3000.10.48 (7e8e528a)"
	}
	path, found := FindDevin(nil)
	choices := DevinChoices(found)
	if path != "" || len(choices) != 2 {
		t.Fatalf("path %q, candidates %+v", path, found)
	}
	if i := LikelyCurrent(choices); i < 0 || !strings.HasPrefix(choices[i].Path, current) {
		t.Fatalf("likely %d of %+v", i, choices)
	}
	msg := DevinMissing(found, "Devin 3.10.48")
	if !strings.Contains(msg, "More than one Devin command line works") || !strings.Contains(msg, "(devin 3000.10.48 (7e8e528a)), likely the current one") || strings.Contains(msg, "Install Devin CLI") {
		t.Fatalf("message %q", msg)
	}
}

// Releases that cannot be told apart suggest neither.
func TestLikelyCurrentNeedsReleasesToCompare(t *testing.T) {
	if i := LikelyCurrent([]DevinCandidate{{Release: "devin 1.2.3"}, {Release: ""}}); i != -1 {
		t.Errorf("an unknown release: %d", i)
	}
	if i := LikelyCurrent([]DevinCandidate{{Release: "devin 1.2.3"}, {Release: "devin 1.2.3"}}); i != -1 {
		t.Errorf("a tie: %d", i)
	}
	if i := LikelyCurrent([]DevinCandidate{{Release: "devin 1.10.0"}, {Release: "devin 1.9.9"}}); i != 0 {
		t.Errorf("1.10.0 against 1.9.9: %d", i)
	}
}

// When no devin is the command line, the message says what to do, naming
// Install Devin CLI when Devin Desktop is installed, and why each was passed
// over.
func TestDevinMissingSaysWhatToDo(t *testing.T) {
	found := []DevinCandidate{{Path: `C:\Devin\Devin.exe`, Problem: "it is a desktop program, not a command line; this is often Devin Desktop itself"}}
	msg := DevinMissing(found, "Devin 3.10.48")
	for _, want := range []string{"Devin 3.10.48 is installed", "Install Devin CLI", "Ctrl+Shift+P", "administrator", `Passed over C:\Devin\Devin.exe: it is a desktop program`} {
		if !strings.Contains(msg, want) {
			t.Errorf("no %q in %q", want, msg)
		}
	}
	if msg := DevinMissing(nil, ""); !strings.Contains(msg, "was not found") || !strings.Contains(msg, "https://docs.devin.ai/cli") {
		t.Errorf("without Devin Desktop: %q", msg)
	}
}

func TestConsoleProgram(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executables")
	}
	system := filepath.Join(os.Getenv("SystemRoot"), "System32")
	if ok, err := consoleProgram(filepath.Join(system, "notepad.exe")); err != nil || ok {
		t.Errorf("notepad: %v %v", ok, err)
	}
	if ok, err := consoleProgram(filepath.Join(system, "where.exe")); err != nil || !ok {
		t.Errorf("where: %v %v", ok, err)
	}
}
