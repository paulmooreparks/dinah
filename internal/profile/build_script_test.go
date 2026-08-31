package profile

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// scripts/build-dinah.ps1 is the operator's inner loop, and since dinah-359 it
// installs the VS Code extension alongside the binaries so that the editor and
// the binary it talks to cannot drift apart. The tests below run the real
// script against a stand-in repository rather than against this one, because
// the behaviour under test is a dozen lines of PowerShell and a real run would
// spend a full Go build and the extension's whole toolchain to reach them.

// stubCode says which stand-in for the editor's own command a run gets.
type stubCode int

const (
	// codeAbsent leaves the command off PATH entirely, the state of a machine
	// where VS Code has never added its shell command.
	codeAbsent stubCode = iota
	// codeWorks records the arguments it was handed and reports success.
	codeWorks
	// codeFails records the same and reports failure, the shape a stale
	// extension directory takes when it defeats an install.
	codeFails
)

// buildDinahRun is what one run of the script leaves behind for a test to read.
type buildDinahRun struct {
	output  string
	err     error
	repo    string
	binDir  string
	codeLog string
}

// buildDinahStubRepo assembles the smallest tree scripts/build-dinah.ps1 will
// accept: a Go module with one command under cmd/, and an extension directory
// whose package script writes the archive the script goes on to install.
func buildDinahStubRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating the directory for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", rel, err)
		}
	}

	// The module names the same Go release this repository does, so building
	// the stub cannot send the toolchain looking for another one.
	write("go.mod", "module buildscriptstub\n\n"+goDirectiveOfThisRepo(t)+"\n")
	write("cmd/dinah/main.go", "package main\n\nfunc main() {}\n")
	write("editors/vscode/package.json", `{
  "name": "build-script-stub",
  "version": "0.0.0",
  "private": true,
  "scripts": {
    "package": "node scripts/package.mjs"
  }
}
`)
	write("editors/vscode/scripts/package.mjs", `import { mkdirSync, writeFileSync } from "node:fs";

mkdirSync("vsix", { recursive: true });
writeFileSync("vsix/dinah-universal.vsix", "stand-in archive");
`)
	// An empty node_modules is enough to stand for an installed toolchain,
	// because the script only asks whether the directory is there.
	if err := os.MkdirAll(filepath.Join(root, "editors", "vscode", "node_modules"), 0o755); err != nil {
		t.Fatalf("creating the stand-in node_modules: %v", err)
	}

	// The script asks git which commit it is building, so the stand-in is a
	// repository with one commit in it rather than a plain directory. The
	// identity is supplied per command, because a machine whose git has no
	// configured user would otherwise refuse the commit.
	for _, args := range [][]string{
		{"init", "--quiet"},
		{
			"-c", "user.name=build-dinah test",
			"-c", "user.email=build-dinah@example.invalid",
			"commit", "--quiet", "--allow-empty", "-m", "stand-in repository",
		},
	} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Skipf("this machine cannot build a stand-in git repository (%v): %s", err, output)
		}
	}
	return root
}

// goDirectiveOfThisRepo reads the go line out of this repository's own go.mod
// rather than composing one from runtime.Version, whose string carries
// suffixes a go.mod will not parse.
func goDirectiveOfThisRepo(t *testing.T) string {
	t.Helper()

	source, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	for _, line := range strings.Split(string(source), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "go ") {
			return trimmed
		}
	}
	t.Fatal("go.mod carries no go directive, so the stand-in module cannot name a release")
	return ""
}

// runBuildDinah runs the real script against a stand-in repository, with a PATH
// built from named tools rather than inherited. Inheriting it would decide the
// codeAbsent case by whatever this machine happens to have installed.
func runBuildDinah(t *testing.T, code stubCode, args ...string) buildDinahRun {
	t.Helper()

	if runtime.GOOS != "windows" {
		t.Skip("scripts/build-dinah.ps1 targets Windows PowerShell")
	}
	psExe, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skip("this machine has no powershell.exe, which scripts/build-dinah.ps1 needs")
	}

	var pathDirs []string
	for _, tool := range []string{"go", "git", "node", "npm"} {
		found, lookErr := exec.LookPath(tool)
		if lookErr != nil {
			t.Skipf("this machine has no %s, which scripts/build-dinah.ps1 needs", tool)
		}
		pathDirs = append(pathDirs, filepath.Dir(found))
	}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		pathDirs = append(pathDirs, filepath.Join(systemRoot, "System32"))
	}

	run := buildDinahRun{
		repo:    buildDinahStubRepo(t),
		binDir:  t.TempDir(),
		codeLog: filepath.Join(t.TempDir(), "code-arguments.txt"),
	}

	if code != codeAbsent {
		exitCode := "0"
		if code == codeFails {
			exitCode = "1"
		}
		stubDir := t.TempDir()
		stub := "@echo off\r\n>>\"%DINAH_CODE_LOG%\" echo %*\r\nexit /b " + exitCode + "\r\n"
		if err := os.WriteFile(filepath.Join(stubDir, "code.cmd"), []byte(stub), 0o644); err != nil {
			t.Fatalf("writing the stand-in code command: %v", err)
		}
		pathDirs = append([]string{stubDir}, pathDirs...)
	}

	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "build-dinah.ps1"))
	if err != nil {
		t.Fatalf("resolving the script under test: %v", err)
	}
	invocation := append([]string{
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script,
		"-SkipPull", "-Repo", run.repo, "-BinDir", run.binDir,
	}, args...)

	cmd := exec.Command(psExe, invocation...)
	cmd.Env = windowsPowerShellEnv(
		"PATH="+strings.Join(pathDirs, string(os.PathListSeparator)),
		"DINAH_CODE_LOG="+run.codeLog,
	)
	output, err := cmd.CombinedOutput()
	run.output = string(output)
	run.err = err
	return run
}

// vsix reports whether the packaging step wrote its archive.
func (run buildDinahRun) vsix() bool {
	_, err := os.Stat(filepath.Join(run.repo, "editors", "vscode", "vsix", "dinah-universal.vsix"))
	return err == nil
}

// codeArguments returns what the stand-in editor command was handed, or the
// empty string when it was never called.
func (run buildDinahRun) codeArguments(t *testing.T) string {
	t.Helper()

	recorded, err := os.ReadFile(run.codeLog)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("reading what the stand-in code command was handed: %v", err)
	}
	return string(recorded)
}

// binaryInstalled reports whether the Go half of the run finished, which every
// case below asserts because no extension outcome is allowed to cost it.
func (run buildDinahRun) binaryInstalled() bool {
	_, err := os.Stat(filepath.Join(run.binDir, "dinah.exe"))
	return err == nil
}

// TestBuildDinahPackagesAndInstallsTheExtension covers the default: nothing on
// the command line asks for the extension, and the extension is built and
// installed anyway.
func TestBuildDinahPackagesAndInstallsTheExtension(t *testing.T) {
	run := runBuildDinah(t, codeWorks)
	if run.err != nil {
		t.Fatalf("the script failed: %v\n%s", run.err, run.output)
	}
	if !run.binaryInstalled() {
		t.Errorf("no binary was installed:\n%s", run.output)
	}
	if !run.vsix() {
		t.Errorf("the extension was never packaged:\n%s", run.output)
	}

	handed := run.codeArguments(t)
	if handed == "" {
		t.Fatalf("the editor's command was never called, so the extension was not installed:\n%s", run.output)
	}
	if !strings.Contains(handed, "--install-extension") {
		t.Errorf("the editor's command was called without --install-extension: %q", handed)
	}
	if !strings.Contains(handed, "dinah-universal.vsix") {
		t.Errorf("the editor's command was not pointed at the packaged archive: %q", handed)
	}
	// The extension's version number does not change between local builds, so
	// an install without --force stops to ask whether to replace a copy that is
	// already there, which an unattended run cannot answer.
	if !strings.Contains(handed, "--force") {
		t.Errorf("the editor's command was called without --force: %q", handed)
	}
}

// TestBuildDinahSkipsTheExtensionOnRequest covers -SkipExtension, the way out
// for a run that is iterating on Go alone.
func TestBuildDinahSkipsTheExtensionOnRequest(t *testing.T) {
	run := runBuildDinah(t, codeWorks, "-SkipExtension")
	if run.err != nil {
		t.Fatalf("the script failed: %v\n%s", run.err, run.output)
	}
	if !run.binaryInstalled() {
		t.Errorf("no binary was installed:\n%s", run.output)
	}
	if run.vsix() {
		t.Errorf("-SkipExtension packaged the extension anyway:\n%s", run.output)
	}
	if handed := run.codeArguments(t); handed != "" {
		t.Errorf("-SkipExtension called the editor's command anyway: %q", handed)
	}
	if !strings.Contains(run.output, "Skipped the VS Code extension") {
		t.Errorf("the run never said the extension was skipped:\n%s", run.output)
	}
}

// TestBuildDinahSurvivesAMissingCodeCommand holds the promise that a machine
// without the editor's shell command still gets its binaries.
func TestBuildDinahSurvivesAMissingCodeCommand(t *testing.T) {
	run := runBuildDinah(t, codeAbsent)
	if run.err != nil {
		t.Fatalf("a missing code command failed the whole run: %v\n%s", run.err, run.output)
	}
	if !run.binaryInstalled() {
		t.Errorf("a missing code command cost the binary install:\n%s", run.output)
	}
	if run.vsix() {
		t.Errorf("the extension was packaged even though nothing could install it:\n%s", run.output)
	}
	if !strings.Contains(run.output, "'code' command is not on this PATH") {
		t.Errorf("the run never said the editor's command was missing:\n%s", run.output)
	}
	if !strings.Contains(run.output, "Shell Command: Install code command in PATH") {
		t.Errorf("the run never said how to get the editor's command:\n%s", run.output)
	}
}

// TestBuildDinahExplainsAFailedExtensionInstall holds the other half of that
// promise: an install that fails says what to do about it, and says plainly
// that the binaries are not what went wrong.
func TestBuildDinahExplainsAFailedExtensionInstall(t *testing.T) {
	run := runBuildDinah(t, codeFails)
	if run.err == nil {
		t.Fatalf("a failed extension install was reported as success:\n%s", run.output)
	}
	if !run.binaryInstalled() {
		t.Errorf("a failed extension install cost the binary install:\n%s", run.output)
	}
	if !strings.Contains(run.output, "The binaries above are") {
		t.Errorf("the failure never said the binaries were fine:\n%s", run.output)
	}
	if !strings.Contains(run.output, "Close every VS Code window and run this again") {
		t.Errorf("the failure never said what to do about it:\n%s", run.output)
	}
}
