package main

import (
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/verb"
)

// finderOver is a tuiFinder whose executable is exe, whose links resolve to
// resolved, whose PATH answers onPath and pathErr, and in which exactly the
// files in present are programs.
func finderOver(exe, resolved, onPath string, pathErr error, present ...string) tuiFinder {
	return tuiFinder{
		executable: func() (string, error) { return exe, nil },
		resolve:    func(string) (string, error) { return resolved, nil },
		lookPath: func(string) (string, error) {
			if pathErr != nil {
				return onPath, pathErr
			}
			if onPath == "" {
				return "", exec.ErrNotFound
			}
			return onPath, nil
		},
		isProgram: func(path string) bool { return slices.Contains(present, path) },
	}
}

// TestTheLauncherFindsDinahTUI is dinah-603/criteria/39. The finder takes
// dinah-tui from the directory of os.Executable first, then from the
// directory of the executable's resolved path, then from PATH, and counts an
// exec.ErrDot answer as not found. With dinah-tui nowhere, a built dinah
// refuses dinah tui with dinah.tui-missing naming its own directory, exits 2
// and writes nothing to standard output.
func TestTheLauncherFindsDinahTUI(t *testing.T) {
	name := tuiProgram()
	exe := filepath.Join("bin", "dinah")
	beside := filepath.Join("bin", name)
	resolved := filepath.Join("opt", "dinah", "dinah")
	besideResolved := filepath.Join("opt", "dinah", name)
	onPath := filepath.Join("usr", "local", "bin", name)
	cases := []struct {
		name    string
		finder  tuiFinder
		want    string
		wantHit bool
	}{
		{"beside the executable before everywhere else", finderOver(exe, resolved, onPath, nil, beside, besideResolved, onPath), beside, true},
		{"beside the resolved path before PATH", finderOver(exe, resolved, onPath, nil, besideResolved, onPath), besideResolved, true},
		{"on PATH last", finderOver(exe, resolved, onPath, nil, onPath), onPath, true},
		{"not through the current directory", finderOver(exe, resolved, name, exec.ErrDot, name), "", false},
		{"nowhere", finderOver(exe, resolved, "", nil), "", false},
	}
	for _, c := range cases {
		path, first, found := c.finder.find(name)
		if found != c.wantHit || path != c.want {
			t.Errorf("%s: found %v at %q, wanted %v at %q", c.name, found, path, c.wantHit, c.want)
		}
		if first != "bin" {
			t.Errorf("%s: the first directory is %q, wanted bin", c.name, first)
		}
	}

	gobin := goOnPath(t)
	dir := t.TempDir()
	dinah := buildProgram(t, gobin, dir, "dinah", "")
	run := exec.Command(dinah, "tui")
	run.Dir = t.TempDir()
	run.Env = launcherEnviron(t, map[string]string{"PATH": t.TempDir()})
	var stdout, stderr strings.Builder
	run.Stdout, run.Stderr = &stdout, &stderr
	err := run.Run()
	code := exitCodeOf(t, err)
	if code != 2 || stdout.Len() != 0 {
		t.Errorf("with dinah-tui nowhere, dinah tui exited %d and wrote %q to standard output", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "dinah.tui-missing") || !strings.Contains(stderr.String(), dir) {
		t.Errorf("the refusal does not name dinah.tui-missing and %s: %q", dir, stderr.String())
	}
	t.Logf("the refusal: %s", strings.TrimSpace(stderr.String()))
}

// tuiStubSource is a program standing in for dinah-tui. It records the
// arguments it was given, the DINAH_TUI_LAUNCHER it saw and its own process
// identifier in the file STUB_REPORT names, and exits with STUB_EXIT.
const tuiStubSource = `package main

import (
	"encoding/json"
	"os"
	"strconv"
)

func main() {
	code, _ := strconv.Atoi(os.Getenv("STUB_EXIT"))
	launcher, launched := os.LookupEnv("DINAH_TUI_LAUNCHER")
	data, _ := json.Marshal(map[string]any{"args": os.Args[1:], "launcher": launcher, "launched": launched, "pid": os.Getpid()})
	_ = os.WriteFile(os.Getenv("STUB_REPORT"), data, 0o644)
	os.Exit(code)
}
`

// stubReport is what the stub recorded.
type stubReport struct {
	Args     []string `json:"args"`
	Launcher string   `json:"launcher"`
	Launched bool     `json:"launched"`
	PID      int      `json:"pid"`
}

// TestTheLauncherPassesTheExitCode is dinah-603/criteria/40. Against a stub
// dinah-tui beside a built dinah, exiting 0, 2, 4 and 130 in turn, dinah tui
// exits with the stub's code each time; the stub received os.Args[1:]
// unchanged, a global flag written before tui included, and
// DINAH_TUI_LAUNCHER held the launcher's build identity. On POSIX the launch
// replaces the process, so the stub's process identifier is the launcher's.
func TestTheLauncherPassesTheExitCode(t *testing.T) {
	gobin := goOnPath(t)
	dir := t.TempDir()
	dinah := buildProgram(t, gobin, dir, "dinah", "")
	stub := t.TempDir()
	if err := os.WriteFile(filepath.Join(stub, "go.mod"), []byte("module stub\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stub, "main.go"), []byte(tuiStubSource), 0o644); err != nil {
		t.Fatal(err)
	}
	build := exec.Command(gobin, "build", "-o", filepath.Join(dir, tuiProgram()), ".")
	build.Dir = stub
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build of the stub: %v\n%s", err, out)
	}
	info, err := buildinfo.ReadFile(dinah)
	if err != nil {
		t.Fatal(err)
	}
	identity := verb.IdentityOfBuild(info)
	argv := []string{"--quiet", "tui", "mine", "--plain"}
	for _, want := range []int{0, 2, 4, 130} {
		report := filepath.Join(t.TempDir(), "report.json")
		run := exec.Command(dinah, argv...)
		run.Dir = t.TempDir()
		run.Env = launcherEnviron(t, map[string]string{"STUB_EXIT": strconv.Itoa(want), "STUB_REPORT": report})
		err := run.Run()
		if code := exitCodeOf(t, err); code != want {
			t.Errorf("the stub exited %d, and dinah tui exited %d", want, code)
		}
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatalf("the stub left no report for exit %d: %v", want, err)
		}
		var got stubReport
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.Args, argv) {
			t.Errorf("the stub received %q, and dinah was given %q", got.Args, argv)
		}
		if !got.Launched || got.Launcher != identity {
			t.Errorf("the stub saw DINAH_TUI_LAUNCHER %q (set %v), and the launcher's identity is %q", got.Launcher, got.Launched, identity)
		}
		if runtime.GOOS != "windows" && got.PID != run.Process.Pid {
			t.Errorf("the stub ran as process %d, and dinah as %d, so the launch did not replace the process", got.PID, run.Process.Pid)
		}
	}
	t.Logf("the launcher's identity: %s", identity)
}

// goOnPath answers the go command, skipping the test where there is none.
func goOnPath(t *testing.T) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH, so the programs cannot be built")
	}
	return gobin
}

// buildProgram builds ./cmd/dinah with the build tags given into dir under
// name, with the executable suffix of this GOOS, and answers its path.
func buildProgram(t *testing.T, gobin, dir, name, tags string) string {
	t.Helper()
	path := filepath.Join(dir, name+exeSuffix())
	args := []string{"build", "-o", path}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	if out, err := exec.Command(gobin, append(args, ".")...).CombinedOutput(); err != nil {
		t.Fatalf("go build of %s: %v\n%s", name, err, out)
	}
	return path
}

// launcherEnviron is the environment a built program runs under in these
// tests: this process's own, with every DINAH_ variable removed and HOME,
// USERPROFILE and DINAH_HOME pointed at a directory of the test's own, and
// then the variables given set.
func launcherEnviron(t *testing.T, set map[string]string) []string {
	t.Helper()
	home := t.TempDir()
	values := map[string]string{"HOME": home, "USERPROFILE": home, "DINAH_HOME": filepath.Join(home, "dinah-home")}
	for name, value := range set {
		values[name] = value
	}
	var environ []string
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if strings.HasPrefix(strings.ToUpper(name), "DINAH_") {
			continue
		}
		if _, replaced := values[name]; replaced {
			continue
		}
		if runtime.GOOS == "windows" {
			if _, replaced := values[strings.ToUpper(name)]; replaced {
				continue
			}
		}
		environ = append(environ, variable)
	}
	for name, value := range values {
		environ = append(environ, name+"="+value)
	}
	return environ
}

// exitCodeOf answers the exit code a finished command's error carries.
func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the program did not run: %v", err)
	}
	return exit.ExitCode()
}

// exeSuffix is what a built binary's name ends in on this GOOS.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
