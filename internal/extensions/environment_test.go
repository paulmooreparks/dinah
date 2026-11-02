package extensions

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestExtensionEnvironmentUsesAllowListAndSignedDeclarations(t *testing.T) {
	env := []string{"PATH=tools", "SystemRoot=system", "HOME=home", "PRIVATE_PASSWORD=canary-password", "UNEXPECTED_SECRET=canary-secret", "SERVICE_TOKEN=canary-token", "EXPLICIT_SECRET=canary-opt-in", "Path=last-tools", "invalid", "=drive-state", "HOME=bad\x00value"}
	for _, platform := range []string{"windows", "linux", "darwin"} {
		t.Run(platform, func(t *testing.T) {
			got := cleanEnvironmentFor(platform, env, []string{"EXPLICIT_SECRET"})
			text := strings.Join(got, "\n")
			for _, canary := range []string{"canary-password", "canary-secret", "canary-token", "bad\x00value"} {
				if strings.Contains(text, canary) {
					t.Fatal("undeclared or invalid inherited value escaped")
				}
			}
			if !slices.Contains(got, "EXPLICIT_SECRET=canary-opt-in") || !slices.Contains(got, "HOME=home") {
				t.Fatal("declared or standard value missing")
			}
			if platform == "windows" {
				if !slices.Contains(got, "Path=last-tools") || slices.Contains(got, "PATH=tools") {
					t.Fatal("Windows case folding or final duplicate value lost")
				}
			} else if !slices.Contains(got, "PATH=tools") || slices.Contains(got, "Path=last-tools") {
				t.Fatal("Unix environment names were case folded")
			}
			if got := cleanEnvironmentFor(platform, []string{"explicit_secret=hidden"}, []string{"EXPLICIT_SECRET"}); (len(got) == 1) != (platform == "windows") {
				t.Fatal("declared variable name matching differs from policy")
			}
		})
	}
	if got := cleanEnvironment(nil); got == nil || len(got) != 0 {
		t.Fatal("empty input must give an explicit empty child environment")
	}
}

func TestExtensionEnvironmentSupportsGitCommands(t *testing.T) {
	for _, name := range []string{"APPDATA", "LOCALAPPDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)", "SystemDrive", "NUMBER_OF_PROCESSORS", "PROCESSOR_ARCHITECTURE"} {
		entry := name + "=synthetic-location"
		if !slices.Contains(cleanEnvironmentFor("windows", []string{entry}, nil), entry) {
			t.Fatalf("standard Windows name missing: %s", name)
		}
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	home := t.TempDir()
	// Configuration and repository data are temporary. No user hooks, signing,
	// credential helpers, remote, or real credential file participates.
	env := cleanEnvironment(os.Environ())
	env = append(env, "HOME="+home, "USERPROFILE="+home, "XDG_CONFIG_HOME="+home, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(home, "empty-config"))
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	run("init", "--quiet")
	if err := os.WriteFile(filepath.Join(dir, "example.txt"), []byte("test content"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "--", "example.txt")
	if got := run("diff", "--cached", "--name-only"); strings.TrimSpace(got) != "example.txt" {
		t.Fatalf("staged diff: %q", got)
	}
	if got := run("status", "--porcelain"); !strings.Contains(got, "A  example.txt") {
		t.Fatalf("status: %q", got)
	}
}
