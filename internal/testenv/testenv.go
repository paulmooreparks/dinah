// Package testenv isolates a test binary's temporary-directory location from
// the developer's own home, so a package whose tests exercise an
// ancestor-walking search cannot reach real data sitting above the fixture
// tree those tests build.
//
// The protection covers exactly what t.TempDir() builds. Every test in the
// two packages that call IsolateTempDir constructs its fixtures exclusively
// through that helper, so redirecting TMP, TEMP, and TMPDIR before any test
// runs redirects every fixture tree along with it. A future test that
// constructs a fixture path some other way, a literal path under the
// profile for instance, sits outside this protection; wiring the same
// TestMain into a new package only covers the tests in that package's own
// binary.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
)

// IsolateTempDir points TMP, TEMP, and TMPDIR at a directory outside the
// user's home for the rest of the process, so every t.TempDir() call in the
// running test binary resolves there instead of under the operating
// system's default. Call it from TestMain, before m.Run(), in any package
// whose tests call bench.Discover or bench.Reachable; a package that only
// opens a bench at an explicit root (bench.Open) takes no ancestor walk and
// needs no isolation.
//
// It is a no-op, and returns a no-op restore, when the operating system's
// default temporary directory is already outside home, which is the case on
// every POSIX default and is why this changes nothing on the ubuntu-latest
// and macos-latest legs of CI.
//
// It prints where it looked, what it expected, and what to set instead, then
// exits the process, when it cannot produce or verify a directory outside
// home. A test suite that cannot prove its own isolation must not run
// unisolated and report a pass.
func IsolateTempDir() (restore func()) {
	noop := func() {}

	home, err := os.UserHomeDir()
	if err != nil {
		// Nothing to protect: bench.NativeHome() treats the same error the
		// same way, as an unbounded walk rather than a guess at a boundary.
		return noop
	}

	original := os.TempDir()
	if !decideIsolation(original, home) {
		return noop
	}

	root := filepath.VolumeName(original) + string(filepath.Separator)
	created, mkdirErr := os.MkdirTemp(root, "dinah-testenv-*")
	if ok, message := verifyIsolation(original, home, created, mkdirErr); !ok {
		fmt.Fprintln(os.Stderr, message)
		os.Exit(1)
	}

	prevTMP, hadTMP := os.LookupEnv("TMP")
	prevTEMP, hadTEMP := os.LookupEnv("TEMP")
	prevTMPDIR, hadTMPDIR := os.LookupEnv("TMPDIR")

	os.Setenv("TMP", created)
	os.Setenv("TEMP", created)
	os.Setenv("TMPDIR", created)

	return func() {
		restoreEnv("TMP", prevTMP, hadTMP)
		restoreEnv("TEMP", prevTEMP, hadTEMP)
		restoreEnv("TMPDIR", prevTMPDIR, hadTMPDIR)
		os.RemoveAll(created)
	}
}

// decideIsolation is the pure decision behind IsolateTempDir: whether the
// process's default temporary directory needs redirecting at all, given the
// directory itself and the home it must sit outside of. Kept separate from
// IsolateTempDir so a unit test can supply both inputs directly rather than
// depend on which operating system the test runner happens to be.
func decideIsolation(defaultTemp, home string) bool {
	return insideHome(defaultTemp, home)
}

// verifyIsolation decides whether a directory IsolateTempDir just created can
// be trusted as isolation, and if not, the message the process should print
// before exiting. Kept separate from the os.Exit call it backs so a unit
// test can exercise the refusal without forcing a real process exit.
func verifyIsolation(defaultTemp, home, created string, mkdirErr error) (ok bool, message string) {
	if mkdirErr != nil {
		return false, failMessage(defaultTemp, home, fmt.Errorf("could not create a directory to isolate into: %w", mkdirErr))
	}
	if insideHome(created, home) {
		return false, failMessage(defaultTemp, home, fmt.Errorf("the directory created at %s is still inside %s", created, home))
	}
	return true, ""
}

// failMessage names where IsolateTempDir looked, what it expected to find,
// and what to set instead, in the shape every refusal on this board follows:
// where the tool looked, what it expected, what to do next. A contributor
// who trips this is by definition somebody whose environment surprised us.
func failMessage(defaultTemp, home string, cause error) string {
	return fmt.Sprintf(
		"testenv: looked for a temporary directory outside %s, starting from the operating system default %s, and failed: %v\n"+
			"testenv: expected TMP, TEMP, and TMPDIR to resolve outside your home directory before any test runs, so a fixture tree cannot climb into it\n"+
			"testenv: set TMP and TEMP (and TMPDIR too, on a system that reads it) to a directory outside %s yourself, then run the tests again",
		home, defaultTemp, cause, home,
	)
}

// restoreEnv puts an environment variable back the way IsolateTempDir found
// it: set to its prior value when it had one, or unset entirely when it did
// not.
func restoreEnv(key, prev string, had bool) {
	if had {
		os.Setenv(key, prev)
		return
	}
	os.Unsetenv(key)
}

// insideHome reports whether dir names home or a descendant of home, using a
// path-segment boundary rather than a string prefix, so a sibling directory
// whose name happens to start with home's name (/home/paulson against
// /home/paul) does not read as inside it.
func insideHome(dir, home string) bool {
	if dir == "" || home == "" {
		return false
	}
	dir = filepath.Clean(dir)
	home = filepath.Clean(home)
	if dir == home {
		return true
	}
	rel, err := filepath.Rel(home, dir)
	if err != nil {
		return false
	}
	return !hasParentSegment(rel)
}

// hasParentSegment reports whether a relative path climbs above the
// directory it was computed against, meaning it starts with a ".." segment.
// filepath.Rel produces exactly this shape for any path outside its base, so
// this is what turns "the relative path exists" into "the relative path
// still lands inside".
func hasParentSegment(rel string) bool {
	return rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}
