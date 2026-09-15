package mcp

import (
	"os"
	"testing"

	"dinah/internal/testenv"
)

// TestMain clears the variables isolatedEnv names for the whole run, so a
// shell that exports one does not reach a test that never asked to see it.
// dinah-496.
//
// This package had no TestMain before dinah-496, and needed none: nothing it
// read from the environment before that card was something a developer's
// shell was likely to carry. dinah-496 gave mcp.go's own call to
// bench.ResolveAgent a reason to run on every write this package's tests
// make, the same call cmd/dinah's TestMain already isolates its own binary
// from. A round of Test caught the gap by exporting the four names to
// illegal values and watching writes this package's tests never meant to be
// about identity refuse with dinah.malformed-harness.
func TestMain(m *testing.M) {
	restoreIsolated := testenv.ClearVars(isolatedEnv...)
	code := m.Run()
	restoreIsolated()
	os.Exit(code)
}

// isolatedEnv names the variables this package's own resolvers read straight
// from the environment and that no test here asked to see. dinah-496.
//
// It is a twin of cmd/dinah's and internal/bench's own lists rather than a
// reference to either: cmd/dinah is package main and nothing can import it,
// and the rule the three lists share is that a list names what its own
// package reads, so a list here that named COLUMNS or the editor variables
// would claim an isolation this package's production code does not need.
// mcp.go's own call to bench.ResolveAgent is the only thing this package
// reads from the environment, so the list carries exactly the four names
// that call resolves: DINAH_HARNESS, DINAH_PROVIDER, DINAH_MODEL and
// DINAH_SERVER.
var isolatedEnv = []string{
	"DINAH_HARNESS", "DINAH_PROVIDER", "DINAH_MODEL", "DINAH_SERVER",
}

// TestIsolatedEnvNamesEveryVariableTheBinaryClears is the twin of the test of
// the same name in cmd/dinah and internal/bench, over this package's own
// list. dinah-496.
//
// The first half compares isolatedEnv against a copy written out here, which
// catches a name dropped from isolatedEnv later and fails on any machine,
// including a CI runner exporting none of the four. The second half asserts
// that TestMain actually made the call, by reading each name back while
// tests run, which only fails on a machine that exports one of the four.
// Both halves are needed for the same reason the two other packages carry
// both: without the first, a dropped name is invisible on CI; without the
// second, a list nothing acts on passes.
func TestIsolatedEnvNamesEveryVariableTheBinaryClears(t *testing.T) {
	want := []string{
		"DINAH_HARNESS", "DINAH_PROVIDER", "DINAH_MODEL", "DINAH_SERVER",
	}
	for _, name := range want {
		if !namesVariable(isolatedEnv, name) {
			t.Errorf("isolatedEnv no longer names %s, so this binary inherits it from whoever runs the tests", name)
		}
	}
	for _, name := range isolatedEnv {
		if !namesVariable(want, name) {
			t.Errorf("isolatedEnv names %s, which this test does not expect: add it here with the reason, or take it out of the list", name)
		}
	}
	if len(isolatedEnv) != len(want) {
		t.Errorf("isolatedEnv carries %d names, wanted %d: %v", len(isolatedEnv), len(want), isolatedEnv)
	}

	for _, name := range want {
		if value, set := os.LookupEnv(name); set {
			t.Errorf("%s is still set to %q while tests run, so TestMain did not clear it", name, value)
		}
	}
}

// namesVariable reports whether a list of environment variable names carries
// one, which is what lets the guard above name the variable that went missing
// rather than print two slices and leave the reader to diff them. A twin of
// the same helper in cmd/dinah and internal/bench, kept local for the same
// reason those two stay separate from each other.
func namesVariable(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
