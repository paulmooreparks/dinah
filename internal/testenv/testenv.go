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
	"runtime"
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
// exits the process, when it cannot establish a home directory to compare
// against at all (os.UserHomeDir fails, or the path it names does not stand
// up as a real, readable directory), or when it cannot produce or verify a
// directory outside home once it has one. A test suite that cannot prove its
// own isolation must not run unisolated and report a pass; a case it cannot
// evaluate is exactly the case where an unprotected run could reach real
// data, not a case with nothing to protect.
func IsolateTempDir() (restore func()) {
	noop := func() {}

	home, homeErr := os.UserHomeDir()
	var statErr error
	if homeErr == nil {
		info, err := os.Stat(home)
		switch {
		case err != nil:
			statErr = err
		case !info.IsDir():
			statErr = fmt.Errorf("%s is not a directory", home)
		}
	}
	if ok, message := resolveHome(home, homeErr, statErr); !ok {
		fmt.Fprintln(os.Stderr, message)
		os.Exit(1)
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

// resolveHome decides whether IsolateTempDir has a home directory it can
// trust as a boundary, given the raw results of resolving it
// (os.UserHomeDir) and confirming it stands up as a real, readable directory
// (os.Stat). Kept separate from those two calls so a unit test can supply
// both outcomes directly rather than depend on which home this machine
// actually has, the same reason decideIsolation and verifyIsolation stay
// separate from the filesystem calls that feed them. A resolution error
// takes priority in the message when both are present, since a stat was
// never attempted without a resolved path to stat.
func resolveHome(home string, homeErr, statErr error) (ok bool, message string) {
	if homeErr != nil {
		return false, unresolvedHomeMessage(homeErr)
	}
	if statErr != nil {
		return false, unverifiedHomeMessage(home, statErr)
	}
	return true, ""
}

// homeEnvVar names the environment variable os.UserHomeDir reads to resolve
// home on this platform, so a refusal can tell the reader exactly which
// variable to set rather than making them guess.
func homeEnvVar() string {
	if runtime.GOOS == "windows" {
		return "USERPROFILE"
	}
	return "HOME"
}

// unresolvedHomeMessage is the refusal for an os.UserHomeDir error: the
// guard could not establish where home is at all, so it has no boundary to
// compare the default temporary directory against and cannot tell whether a
// test run would reach real data. Follows the same where-looked,
// what-expected, what-to-set shape as failMessage.
func unresolvedHomeMessage(cause error) string {
	v := homeEnvVar()
	return fmt.Sprintf(
		"testenv: could not resolve your home directory, so there is no boundary to isolate temporary files from: %v\n"+
			"testenv: expected %s to be set, on this platform, to your real home directory before any test runs\n"+
			"testenv: set %s to your home directory yourself, then run the tests again",
		cause, v, v,
	)
}

// unverifiedHomeMessage is the refusal for a stat failure on a resolved
// home: os.UserHomeDir reported no error, but the path it named does not
// stand up as a real, readable directory, so the guard has a string with
// nothing behind it rather than a boundary it can trust. Follows the same
// where-looked, what-expected, what-to-set shape as failMessage.
func unverifiedHomeMessage(home string, cause error) string {
	v := homeEnvVar()
	return fmt.Sprintf(
		"testenv: looked for your home directory at %s (resolved from %s), and could not confirm it as a real, readable directory: %v\n"+
			"testenv: expected %s to name a directory that actually exists and is readable before any test runs\n"+
			"testenv: set %s to a real, existing directory yourself, then run the tests again",
		home, v, cause, v, v,
	)
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
