package testenv

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// insideHomeCases covers an identical path, a nested descendant, a sibling
// whose name shares a text prefix with home, and a path outside home
// entirely, all against constructed strings so the test needs no real
// filesystem or environment.
var insideHomeCases = []struct {
	name string
	dir  string
	home string
	want bool
}{
	{name: "identical", dir: "/home/paul", home: "/home/paul", want: true},
	{name: "nested descendant", dir: "/home/paul/src/project", home: "/home/paul", want: true},
	{name: "prefix-sharing sibling", dir: "/home/paulson", home: "/home/paul", want: false},
	{name: "outside entirely", dir: "/var/tmp", home: "/home/paul", want: false},
}

// TestInsideHomeUsesASegmentBoundary asserts that insideHome answers each of
// the four shapes a caller can hand it correctly, and in particular that a
// sibling directory sharing home's name as a text prefix does not read as
// inside it.
func TestInsideHomeUsesASegmentBoundary(t *testing.T) {
	for _, c := range insideHomeCases {
		got := insideHome(c.dir, c.home)
		if got != c.want {
			t.Errorf("%s: insideHome(%q, %q) = %v, wanted %v", c.name, c.dir, c.home, got, c.want)
		}
	}
}

// TestDecideIsolationIsANoOpOutsideHome asserts that decideIsolation reports
// no isolation needed once the default temporary directory already sits
// outside home, and that it reports isolation needed when it does not,
// against inputs the test fakes directly rather than the operating system
// the test runner happens to be. Both fakes are built with filepath.Join in
// the running OS's own separator convention, so the nesting insideHome reads
// is real nesting on every platform, rather than a Windows-style literal
// that a POSIX build would read as one opaque path segment.
func TestDecideIsolationIsANoOpOutsideHome(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "paul")
	outside := filepath.Join(string(filepath.Separator), "var", "tmp")
	inside := filepath.Join(home, "AppData", "Local", "Temp")

	if got := decideIsolation(outside, home); got {
		t.Errorf("a default temp dir outside home should not need isolation, got %v", got)
	}
	if got := decideIsolation(inside, home); !got {
		t.Errorf("a default temp dir inside home should need isolation, got %v", got)
	}
}

// TestResolveHomeRefusesAnUnresolvedHome asserts that resolveHome refuses,
// rather than reporting nothing to protect, when os.UserHomeDir itself
// failed, and that the refusal names the cause.
func TestResolveHomeRefusesAnUnresolvedHome(t *testing.T) {
	cause := errors.New("%userprofile% is not defined")
	ok, message := resolveHome("", cause, nil)
	if ok {
		t.Fatalf("an unresolved home should not resolve as trustworthy")
	}
	if !strings.Contains(message, cause.Error()) {
		t.Errorf("refusal message should mention the cause %q, got %q", cause.Error(), message)
	}
	for _, want := range []string{homeEnvVar(), "set"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal message should mention %q, got %q", want, message)
		}
	}
}

// TestResolveHomeRefusesAHomeThatFailsStat asserts that resolveHome refuses
// when os.UserHomeDir reported no error but the resolved path failed an
// os.Stat check, and that the refusal names both the path and the cause.
func TestResolveHomeRefusesAHomeThatFailsStat(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "ghost")
	cause := errors.New("no such file or directory")
	ok, message := resolveHome(home, nil, cause)
	if ok {
		t.Fatalf("a home that fails a stat check should not resolve as trustworthy")
	}
	for _, want := range []string{home, cause.Error(), homeEnvVar(), "set"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal message should mention %q, got %q", want, message)
		}
	}
}

// TestResolveHomeAcceptsAResolvedAndStatableHome asserts the success path:
// a home that resolved with no error and stated with no error resolves as
// trustworthy, with no message.
func TestResolveHomeAcceptsAResolvedAndStatableHome(t *testing.T) {
	ok, message := resolveHome("/home/paul", nil, nil)
	if !ok {
		t.Fatalf("a resolved, statable home should resolve as trustworthy, got message %q", message)
	}
	if message != "" {
		t.Errorf("a trustworthy home should carry no message, got %q", message)
	}
}

// TestVerifyIsolationCatchesAnUnhelpfulMkdir asserts that verifyIsolation
// refuses when MkdirTemp itself failed, without needing a real MkdirTemp
// call or a real process exit.
func TestVerifyIsolationCatchesAnUnhelpfulMkdir(t *testing.T) {
	ok, message := verifyIsolation("/tmp", "/home/paul", "", errors.New("disk full"))
	if ok {
		t.Fatalf("a MkdirTemp failure should not verify as isolated")
	}
	if message == "" {
		t.Errorf("a refusal should carry a message naming what went wrong")
	}
}

// TestVerifyIsolationCatchesADirectoryStillInsideHome asserts that
// verifyIsolation refuses a created directory that did not actually land
// outside home, which is what makes the check verify the result rather than
// merely that a directory got created.
func TestVerifyIsolationCatchesADirectoryStillInsideHome(t *testing.T) {
	ok, message := verifyIsolation("/home/paul/tmp", "/home/paul", "/home/paul/tmp/dinah-testenv-1", nil)
	if ok {
		t.Fatalf("a directory still inside home should not verify as isolated")
	}
	for _, want := range []string{"/home/paul", "looked", "expected", "set TMP"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal message should mention %q, got %q", want, message)
		}
	}
}

// TestVerifyIsolationAcceptsADirectoryOutsideHome asserts the success path:
// a directory that both landed outside home and carries no MkdirTemp error
// verifies with no message.
func TestVerifyIsolationAcceptsADirectoryOutsideHome(t *testing.T) {
	ok, message := verifyIsolation("/home/paul/tmp", "/home/paul", "/var/tmp/dinah-testenv-1", nil)
	if !ok {
		t.Fatalf("a directory outside home should verify as isolated, got message %q", message)
	}
	if message != "" {
		t.Errorf("a verified isolation should carry no message, got %q", message)
	}
}

// TestIsolateTempDirRedirectsAndRestores calls the real IsolateTempDir and
// its restore in-process against this machine's real home and real default
// temporary directory, and checks both the filesystem and the environment
// afterward: the created directory exists while isolated and is gone after
// restore, and TMP, TEMP, and TMPDIR return to what they held before.
//
// This deliberately does not fake home to force the redirect branch, the
// way TestIsolateTempDirIsANoOpOutsideHome fakes the no-op branch. The
// branch this test exercises ends in a real os.MkdirTemp rooted at the
// filesystem's own root (never a fixed path, per this package's design), and
// a POSIX account with no home nested under that root cannot create a
// directory there; forcing the branch with a fake home on such a machine
// would fail on a permission this test has no business asserting about. The
// real home and real default answer the same question IsolateTempDir itself
// answers, so the skip below is the same no-op this function already
// returns on a machine where nothing needs isolating.
func TestIsolateTempDirRedirectsAndRestores(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory to test against on this machine")
	}
	before := os.TempDir()
	if !insideHome(before, home) {
		t.Skip("this machine's default temporary directory already sits outside home; see TestIsolateTempDirIsANoOpOutsideHome for that branch")
	}

	prevTMP := os.Getenv("TMP")
	prevTEMP := os.Getenv("TEMP")
	prevTMPDIR := os.Getenv("TMPDIR")

	restore := IsolateTempDir()

	created := os.Getenv("TMP")
	if created == prevTMP {
		t.Fatalf("TMP was not redirected away from %s", prevTMP)
	}
	if insideHome(created, home) {
		t.Errorf("the redirected directory %s is still inside home %s", created, home)
	}
	if os.Getenv("TEMP") != created || os.Getenv("TMPDIR") != created {
		t.Errorf("TEMP and TMPDIR should match TMP's redirected value %s, got TEMP=%s TMPDIR=%s", created, os.Getenv("TEMP"), os.Getenv("TMPDIR"))
	}
	if _, err := os.Stat(created); err != nil {
		t.Fatalf("the redirected directory should exist while isolated: %v", err)
	}

	restore()

	if os.Getenv("TMP") != prevTMP || os.Getenv("TEMP") != prevTEMP || os.Getenv("TMPDIR") != prevTMPDIR {
		t.Errorf("restore should put TMP, TEMP, and TMPDIR back to what they held before, got TMP=%s TEMP=%s TMPDIR=%s", os.Getenv("TMP"), os.Getenv("TEMP"), os.Getenv("TMPDIR"))
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Errorf("restore should remove the directory it created, got err=%v", err)
	}
}

// TestIsolateTempDirIsANoOpOutsideHome asserts the real IsolateTempDir takes
// no action, and its restore does nothing, when the fake default temporary
// directory it is handed already sits outside the fake home.
func TestIsolateTempDirIsANoOpOutsideHome(t *testing.T) {
	fakeHome := t.TempDir()
	outside := t.TempDir()
	if insideHome(outside, fakeHome) {
		t.Fatalf("test setup: %s should not read as inside %s", outside, fakeHome)
	}

	homeVar := "HOME"
	if runtime.GOOS == "windows" {
		homeVar = "USERPROFILE"
	}
	t.Setenv(homeVar, fakeHome)
	t.Setenv("TMP", outside)
	t.Setenv("TEMP", outside)
	t.Setenv("TMPDIR", outside)

	restore := IsolateTempDir()
	if os.Getenv("TMP") != outside {
		t.Errorf("a no-op should leave TMP untouched, got %s", os.Getenv("TMP"))
	}
	restore()
	if os.Getenv("TMP") != outside {
		t.Errorf("a no-op restore should still leave TMP untouched, got %s", os.Getenv("TMP"))
	}
}

// TestHelperProcess is not a test of its own. It is the subprocess entry
// point the two tests below spawn, following the standard library's own
// idiom for exercising a code path that ends in os.Exit from inside a test
// binary (see os/exec_test.go): go test already runs the compiled test
// binary as its own process, so an os.Exit call inside it kills only that
// child process and never touches the shell that invoked go test. Every
// environment value this subprocess sees is built explicitly by
// runIsolateTempDirHelper below, never inherited from the real profile
// running the outer suite, which is what makes it safe to call the real
// IsolateTempDir here.
//
// Guarded behind a marker environment variable so a plain `go test ./...`
// run, which executes every Test-prefixed function by default, does not
// call the real IsolateTempDir a second time and exit the whole suite.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("DINAH_TESTENV_WANT_HELPER_PROCESS") != "1" {
		return
	}
	IsolateTempDir()
	// Every scenario runIsolateTempDirHelper builds is one IsolateTempDir
	// should refuse. Reaching this line means it did not, which the parent
	// test reads as a failure via the exit code, so exit distinctly from
	// the refusal path's os.Exit(1) rather than falling through to a plain
	// return (which would itself exit 0, the same code a passed-through bug
	// produces, but this is clearer about why).
	os.Exit(0)
}

// runIsolateTempDirHelper spawns TestHelperProcess as a real child process
// with an explicitly built environment, so the manipulated home variable
// this test needs never reaches the process running the suite itself. It
// strips HOME, USERPROFILE, HOMEDRIVE, HOMEPATH, TMP, TEMP, and TMPDIR from
// the inherited environment before adding back only what the caller asks
// for, plus TMP/TEMP/TMPDIR pointed at a scratch directory the test owns,
// so that even a broken guard that fails to refuse cannot reach past a
// harmless os.MkdirTemp call under that scratch tree.
func runIsolateTempDirHelper(t *testing.T, homeVar, homeValue string) (exitCode int, stderr string) {
	t.Helper()
	safeTemp := t.TempDir()

	env := make([]string, 0, len(os.Environ())+4)
	for _, kv := range os.Environ() {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		switch strings.ToUpper(key) {
		case "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "TMP", "TEMP", "TMPDIR":
			continue
		default:
			env = append(env, kv)
		}
	}
	env = append(env,
		"DINAH_TESTENV_WANT_HELPER_PROCESS=1",
		"TMP="+safeTemp,
		"TEMP="+safeTemp,
		"TMPDIR="+safeTemp,
	)
	if homeVar != "" {
		env = append(env, homeVar+"="+homeValue)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = env
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()

	exitCode = 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("helper process failed to start: %v", err)
		}
	}
	return exitCode, errBuf.String()
}

// TestIsolateTempDirRefusesAnUnresolvedHomeForReal calls the real
// IsolateTempDir, in a real subprocess, with every home-naming environment
// variable removed, and asserts it refuses with exit code 1 and the
// unresolved-home message. Where TestResolveHomeRefusesAnUnresolvedHome
// (above) only proves the pure resolveHome decision is correct given faked
// inputs, this proves IsolateTempDir itself still calls that decision, still
// hands it the unresolved-home branch rather than the stat branch, and
// still turns a refusal into a real process exit, which is the wiring an
// earlier review found nothing in this file actually covered.
func TestIsolateTempDirRefusesAnUnresolvedHomeForReal(t *testing.T) {
	exitCode, stderr := runIsolateTempDirHelper(t, "", "")
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 for an unresolved home, got %d; stderr=%q", exitCode, stderr)
	}
	if !strings.Contains(stderr, "could not resolve your home directory") {
		t.Errorf("expected the unresolved-home message, got stderr=%q", stderr)
	}
	if strings.Contains(stderr, "could not confirm it as a real, readable directory") {
		t.Errorf("got the unverified-home message instead of the unresolved-home one (homeErr/statErr may be swapped); stderr=%q", stderr)
	}
}

// TestIsolateTempDirRefusesAnUnverifiableHomeForReal calls the real
// IsolateTempDir, in a real subprocess, with the platform's home variable
// pointed at a path that does not exist, and asserts it refuses with exit
// code 1 and the unverified-home message rather than the unresolved-home
// one. Complements the test above by covering the second of the two causes
// resolveHome distinguishes: a home that resolved cleanly but does not
// stand up under os.Stat.
func TestIsolateTempDirRefusesAnUnverifiableHomeForReal(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	exitCode, stderr := runIsolateTempDirHelper(t, homeEnvVar(), missing)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 for a home that fails stat, got %d; stderr=%q", exitCode, stderr)
	}
	if !strings.Contains(stderr, "could not confirm it as a real, readable directory") {
		t.Errorf("expected the unverified-home message, got stderr=%q", stderr)
	}
	if strings.Contains(stderr, "could not resolve your home directory") {
		t.Errorf("got the unresolved-home message instead of the unverified-home one (homeErr/statErr may be swapped); stderr=%q", stderr)
	}
	if !strings.Contains(stderr, missing) {
		t.Errorf("expected the refusal to name the path %q it could not verify, got stderr=%q", missing, stderr)
	}
}
