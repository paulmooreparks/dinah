package testenv

import (
	"errors"
	"os"
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
// the test runner happens to be.
func TestDecideIsolationIsANoOpOutsideHome(t *testing.T) {
	if got := decideIsolation("/tmp", "/home/paul"); got {
		t.Errorf("a POSIX default temp dir outside home should not need isolation, got %v", got)
	}
	if got := decideIsolation(`C:\Users\paul\AppData\Local\Temp`, `C:\Users\paul`); !got {
		t.Errorf("a Windows default temp dir inside home should need isolation, got %v", got)
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
// its restore in-process against a fake home and a fake default temporary
// directory nested inside it, set through the environment variables both
// os.UserHomeDir and os.TempDir read, and checks both the filesystem and the
// environment afterward: the created directory exists while isolated and is
// gone after restore, and TMP, TEMP, and TMPDIR return to what they held
// before.
func TestIsolateTempDirRedirectsAndRestores(t *testing.T) {
	fakeHome := t.TempDir()
	fakeDefaultTemp := filepath.Join(fakeHome, "AppData", "Local", "Temp")
	if err := os.MkdirAll(fakeDefaultTemp, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	homeVar := "HOME"
	if runtime.GOOS == "windows" {
		homeVar = "USERPROFILE"
	}
	t.Setenv(homeVar, fakeHome)
	t.Setenv("TMP", fakeDefaultTemp)
	t.Setenv("TEMP", fakeDefaultTemp)
	t.Setenv("TMPDIR", fakeDefaultTemp)

	restore := IsolateTempDir()

	created := os.Getenv("TMP")
	if created == fakeDefaultTemp {
		t.Fatalf("TMP was not redirected away from the fake default %s", fakeDefaultTemp)
	}
	if insideHome(created, fakeHome) {
		t.Errorf("the redirected directory %s is still inside the fake home %s", created, fakeHome)
	}
	if os.Getenv("TEMP") != created || os.Getenv("TMPDIR") != created {
		t.Errorf("TEMP and TMPDIR should match TMP's redirected value %s, got TEMP=%s TMPDIR=%s", created, os.Getenv("TEMP"), os.Getenv("TMPDIR"))
	}
	if _, err := os.Stat(created); err != nil {
		t.Fatalf("the redirected directory should exist while isolated: %v", err)
	}

	restore()

	if os.Getenv("TMP") != fakeDefaultTemp || os.Getenv("TEMP") != fakeDefaultTemp || os.Getenv("TMPDIR") != fakeDefaultTemp {
		t.Errorf("restore should put TMP, TEMP, and TMPDIR back to %s, got TMP=%s TEMP=%s TMPDIR=%s", fakeDefaultTemp, os.Getenv("TMP"), os.Getenv("TEMP"), os.Getenv("TMPDIR"))
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
